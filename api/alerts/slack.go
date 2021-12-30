package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

const (
	defaultTimeout = 5 * time.Second
	webhookEnv     = "SLACK_WEBHOOK"
)

func postToSlack(text string) error {
	msg := struct {
		Text string `json:"text"`
	}{
		Text: text,
	}
	buf, _ := json.Marshal(msg)
	url, ok := os.LookupEnv(webhookEnv)
	if !ok {
		return fmt.Errorf("slack webhook URL not found")
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(buf))
	if err != nil {
		return fmt.Errorf("new request failed: %s", err)
	}
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{Timeout: defaultTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %s", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response failed: %s", err)
	}
	if string(body) != "ok" {
		return fmt.Errorf("non-ok response: %s", string(body))
	}
	return nil
}
