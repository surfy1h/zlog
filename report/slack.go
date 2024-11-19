package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Reporter interface {
	Report(messages []string) error
}

type SlackMessage struct {
	Text string `json:"text"`
}

type SlackReporter struct {
	config     ReportConfig
	httpClient *http.Client
}

func NewSlackReporter(config ReportConfig) Reporter {
	return &SlackReporter{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Second * 5,
		},
	}
}

func (s *SlackReporter) Report(messages []string) error {
	if len(messages) == 0 {
		return nil
	}

	// Combine all messages with newlines
	combinedMessage := ""
	for i, msg := range messages {
		if i > 0 {
			combinedMessage += "\n"
		}
		combinedMessage += msg
	}

	// Create Slack message payload
	slackMsg := SlackMessage{
		Text: fmt.Sprintf("```\n%s\n```", combinedMessage), // Format as code block
	}

	payload, err := json.Marshal(slackMsg)
	if err != nil {
		return fmt.Errorf("marshal slack message failed: %v", err)
	}

	// Send request to Slack webhook
	req, err := http.NewRequest(http.MethodPost, s.config.Token, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("create request failed: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack api returned non-200 status code: %d", resp.StatusCode)
	}

	return nil
}
