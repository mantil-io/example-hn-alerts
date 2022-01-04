package alerts

import (
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/mantil-io/mantil.go"
	"github.com/microcosm-cc/bluemonday"
)

const (
	lastItemKey     = "last-item"
	sentItemsPrefix = "sent-items"
	usernameEnv     = "HN_USER"
)

const (
	NotificationTypeStoryWithKeywords = iota
	NotificationTypeCommentWithKeywords
	NotificationTypeUserComment
	NotificationTypeCommentOnUserComment
	NotificationTypeUserStory
	NotificationTypeCommentOnUserStory
)

type Alerts struct {
	api       *api
	state     *mantil.KV
	sanitizer *bluemonday.Policy
}

func New() *Alerts {
	state, err := mantil.NewKV("state")
	if err != nil {
		log.Fatal(err)
	}
	return &Alerts{
		api:       &api{},
		state:     state,
		sanitizer: bluemonday.StrictPolicy(),
	}
}

func (a *Alerts) Invoke() error {
	lastItemID, err := a.lastItemID()
	if err != nil {
		log.Println(err)
		return err
	}
	maxItemID, err := a.api.maxItemID()
	if err != nil {
		log.Println(err)
		return err
	}
	if lastItemID == 0 {
		lastItemID = maxItemID - 1000
	}
	for id := lastItemID + 1; id <= maxItemID; id++ {
		i, err := a.api.getItem(id)
		if err != nil {
			log.Println(err)
			continue
		}
		if err := a.processItem(i); err != nil {
			log.Println(err)
			continue
		}
	}
	if err := a.state.Put(lastItemKey, &item{
		ID: maxItemID,
	}); err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (a *Alerts) lastItemID() (int, error) {
	i := &item{}
	err := a.state.Get(lastItemKey, i)
	if errors.As(err, &mantil.ErrItemNotFound{}) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return i.ID, nil
}

func (a *Alerts) processItem(i *item) error {
	if i.Dead || i.Deleted {
		return nil
	}
	return a.processItemRecursive(i, i)
}

func (a *Alerts) processItemRecursive(i *item, root *item) error {
	switch i.Type {
	case "story":
		return a.processStory(i, root)
	case "comment":
		return a.processComment(i, root)
	}
	return nil
}

func (a *Alerts) processStory(s *item, root *item) error {
	if a.isUserItem(s) && s.ID == root.ID {
		if err := a.sendSlackNotification(s, root, s, NotificationTypeUserStory); err != nil {
			return err
		}
	}
	if a.isUserItem(s) && root.Type == "comment" {
		if err := a.sendSlackNotification(s, root, root, NotificationTypeCommentOnUserStory); err != nil {
			return err
		}
	}
	if a.isUserItem(root) && root.Type == "comment" {
		if err := a.sendSlackNotification(s, root, root, NotificationTypeUserComment); err != nil {
			return err
		}
	}
	if a.containsKeywords(s) {
		if err := a.sendSlackNotification(s, root, s, NotificationTypeStoryWithKeywords); err != nil {
			return err
		}
	} else if a.containsKeywords(root) {
		if err := a.sendSlackNotification(s, root, s, NotificationTypeCommentWithKeywords); err != nil {
			return err
		}
	}
	return nil
}

func (a *Alerts) processComment(c *item, root *item) error {
	if a.isUserItem(c) && c.ID != root.ID {
		if err := a.sendSlackNotification(c, root, root, NotificationTypeCommentOnUserComment); err != nil {
			return err
		}
	}
	parent, err := a.api.getItem(c.Parent)
	if err != nil {
		return err
	}
	return a.processItemRecursive(parent, root)
}

func (a *Alerts) isUserItem(i *item) bool {
	u, ok := os.LookupEnv(usernameEnv)
	if !ok {
		return false
	}
	return u == i.By
}

func (a *Alerts) isSent(i *item) bool {
	return a.state.Get(i.sentItemsKey(), i) == nil
}

func (a *Alerts) cleanText(t string) string {
	t = a.sanitizer.Sanitize(t)
	t = strings.ReplaceAll(t, "\n", " ")
	t = strings.ToLower(t)
	linkRegex := regexp.MustCompile(`http\S+`)
	t = linkRegex.ReplaceAllString(t, " ")
	punctuationRegex := regexp.MustCompile(`[!"#$%&'()*+,-.\/:;<=>?@\[\\\]^_` + "`" + `{|}~]`)
	t = punctuationRegex.ReplaceAllString(t, " ")
	return t
}

func (a *Alerts) containsKeywords(i *item) bool {
	text := strings.TrimSpace(i.Title + " " + i.Text)
	text = a.cleanText(text)
	words := strings.Fields(text)
	contains := func(keywords []string) bool {
		for _, w := range words {
			for _, t := range keywords {
				if t == w {
					return true
				}
			}
		}
		return false
	}
	containsLambda := contains([]string{"lambda", "lambdas", "faas"})
	containsGo := contains([]string{"go", "golang"})
	containsServerless := contains([]string{"serverless"})
	return (containsLambda && containsGo) || containsServerless
}

func (a *Alerts) sendSlackNotification(i, root, dedupe *item, typ int) error {
	if dedupe != nil && a.isSent(dedupe) {
		return nil
	}
	text := notificationText(i, root, typ)
	if err := postToSlack(text); err != nil {
		return err
	}
	if dedupe != nil {
		return a.state.Put(dedupe.sentItemsKey(), dedupe)
	}
	return nil
}

func notificationText(i, root *item, typ int) string {
	switch typ {
	case NotificationTypeStoryWithKeywords:
		if i.Title != "" {
			return fmt.Sprintf("A new interesting story was posted on HackerNews:  <https://news.ycombinator.com/item?id=%d|%s>", i.ID, i.Title)
		} else {
			return fmt.Sprintf("A new interesting story was posted on HackerNews:  https://news.ycombinator.com/item?id=%d", i.ID)
		}
	case NotificationTypeCommentWithKeywords:
		if i.Title != "" {
			return fmt.Sprintf("A new interesting <https://news.ycombinator.com/item?id=%d|comment> was posted on a story on HackerNews:  <https://news.ycombinator.com/item?id=%d|%s>", root.ID, i.ID, i.Title)
		} else {
			return fmt.Sprintf("A new interesting comment was posted on a story on HackerNews:  https://news.ycombinator.com/item?id=%d", i.ID)
		}
	case NotificationTypeUserComment:
		return fmt.Sprintf("%s posted a <https://news.ycombinator.com/item?id=%d|comment> on a HackerNews <https://news.ycombinator.com/item?id=%d|story>.", root.By, root.ID, i.ID)
	case NotificationTypeCommentOnUserComment:
		return fmt.Sprintf("%s posted a <https://news.ycombinator.com/item?id=%d|comment> on %s's <https://news.ycombinator.com/item?id=%d|comment> on HackerNews.", root.By, root.ID, i.By, i.ID)
	case NotificationTypeUserStory:
		if i.Title != "" {
			return fmt.Sprintf("%s posted a story on HackerNews: <https://news.ycombinator.com/item?id=%d|%s>", i.By, i.ID, i.Title)
		} else {
			return fmt.Sprintf("%s posted a <https://news.ycombinator.com/item?id=%d|story> on HackerNews.", i.By, i.ID)
		}
	case NotificationTypeCommentOnUserStory:
		return fmt.Sprintf("%s posted a <https://news.ycombinator.com/item?id=%d|comment> on %s's <https://news.ycombinator.com/item?id=%d|story>  on HackerNews.", root.By, root.ID, i.By, i.ID)
	}
	return ""
}
