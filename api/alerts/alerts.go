package alerts

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mantil-io/mantil.go"
	"github.com/microcosm-cc/bluemonday"
)

const (
	lastItemKey     = "last-item"
	sentItemsPrefix = "sent-items"
	apiBaseURL      = "https://hacker-news.firebaseio.com/v0/"
)

type Alerts struct {
	state     *mantil.KV
	sanitizer *bluemonday.Policy
}

func New() *Alerts {
	state, err := mantil.NewKV("state")
	if err != nil {
		log.Fatal(err)
	}
	return &Alerts{
		state:     state,
		sanitizer: bluemonday.StrictPolicy(),
	}
}

type item struct {
	ID      int    `json:"id"`
	Type    string `json:"type,omitempty"`
	Deleted bool   `json:"deleted,omitempty"`
	Dead    bool   `json:"dead,omitempty"`
	Title   string `json:"title,omitempty"`
	Text    string `json:"text,omitempty"`
	Time    int64  `json:"time,omitempty"`
	Parent  int    `json:"parent,omitempty"`
}

func (i *item) sentItemsKey() string {
	return fmt.Sprintf("%s-%d", sentItemsPrefix, i.ID)
}

func (a *Alerts) Invoke() error {
	lastItemID, err := a.lastItemID()
	if err != nil {
		return err
	}
	maxItemID, err := a.maxItemID()
	if err != nil {
		return err
	}
	if lastItemID == 0 {
		lastItemID = maxItemID - 1000
	}
	timeCutoff := time.Now().Add(-10 * time.Minute)
	for id := lastItemID + 1; id <= maxItemID; id++ {
		i, err := a.getItem(id)
		if err != nil {
			return err
		}
		if i.Dead || i.Deleted {
			continue
		}
		if i.Time == 0 || time.Unix(i.Time, 0).Before(timeCutoff) {
			continue
		}
		if !a.isCandidate(i) {
			continue
		}
		if err := a.processItem(i); err != nil {
			return err
		}
	}
	return a.state.Put(lastItemKey, &item{
		ID: maxItemID,
	})
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

func (a *Alerts) maxItemID() (int, error) {
	rsp, err := a.apiCall("maxitem")
	if err != nil {
		return 0, err
	}
	maxItem, err := strconv.Atoi(string(rsp))
	if err != nil {
		return 0, err
	}
	return maxItem, nil
}

func (a *Alerts) getItem(id int) (*item, error) {
	path := fmt.Sprintf("item/%d", id)
	buf, err := a.apiCall(path)
	if err != nil {
		return nil, err
	}
	i := &item{}
	if err := json.Unmarshal(buf, i); err != nil {
		return nil, err
	}
	return i, nil
}

func (a *Alerts) processItem(i *item) error {
	if i.Type == "story" {
		if a.isSent(i) {
			return nil
		}
		return a.sendSlackNotification(i)
	}
	if i.Type == "comment" {
		parent, err := a.getItem(i.Parent)
		if err != nil {
			return err
		}
		return a.processItem(parent)
	}
	return nil
}

func (a *Alerts) isSent(i *item) bool {
	return a.state.Get(i.sentItemsKey(), i) == nil
}

func (a *Alerts) apiCall(path string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s.json", apiBaseURL, path)
	r, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return buf, nil
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

func (a *Alerts) isCandidate(i *item) bool {
	text := strings.TrimSpace(i.Title + " " + i.Text)
	text = a.cleanText(text)
	words := strings.Fields(text)
	contains := func(terms []string) bool {
		for _, w := range words {
			for _, t := range terms {
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

func (a *Alerts) sendSlackNotification(i *item) error {
	var text string
	if i.Title != "" {
		text = fmt.Sprintf("A new interesting story was posted on HackerNews:  <https://news.ycombinator.com/item?id=%d|%s>", i.ID, i.Title)
	} else {
		text = fmt.Sprintf("A new interesting story was posted on HackerNews:  https://news.ycombinator.com/item?id=%d", i.ID)
	}
	if err := postToSlack(text); err != nil {
		return err
	}
	return a.state.Put(i.sentItemsKey(), i)
}
