package alerts

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
)

const apiBaseURL = "https://hacker-news.firebaseio.com/v0/"

type item struct {
	ID      int    `json:"id"`
	Type    string `json:"type,omitempty"`
	Deleted bool   `json:"deleted,omitempty"`
	Dead    bool   `json:"dead,omitempty"`
	Title   string `json:"title,omitempty"`
	Text    string `json:"text,omitempty"`
	Time    int64  `json:"time,omitempty"`
	Parent  int    `json:"parent,omitempty"`
	By      string `json:"by,omitempty"`
}

func (i *item) sentItemsKey() string {
	return fmt.Sprintf("%s-%d", sentItemsPrefix, i.ID)
}

type api struct{}

func (a *api) maxItemID() (int, error) {
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

func (a *api) getItem(id int) (*item, error) {
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

func (a *api) apiCall(path string) ([]byte, error) {
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
