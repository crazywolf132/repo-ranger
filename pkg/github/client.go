package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/crazywolf132/repo-ranger/pkg/types"
)

// Client represents a GitHub API client.
type Client interface {
	PostPRComment(event types.PullRequestEvent, comment string) error
	CreateCheckRun(review string) error
	PostInlineComments(event types.PullRequestEvent, comments []types.InlineComment) error
}

type client struct {
	token      string
	httpClient HTTPClient
}

// HTTPClient represents the interface for making HTTP requests.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// NewClient creates a new GitHub client.
func NewClient(token string, httpClient HTTPClient) Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &client{
		token:      token,
		httpClient: httpClient,
	}
}

func (c *client) PostPRComment(event types.PullRequestEvent, comment string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/comments",
		event.Repository.FullName, event.PullRequest.Number)

	payload := map[string]string{"body": comment}
	return c.postToGitHub(url, payload)
}

func (c *client) CreateCheckRun(review string) error {
	// Implementation would depend on your specific GitHub Check Run requirements
	// This is a placeholder for the actual implementation
	log.Info("Creating GitHub Check Run")
	return nil
}

func (c *client) PostInlineComments(event types.PullRequestEvent, comments []types.InlineComment) error {
	for _, comment := range comments {
		if err := c.postInlineComment(event, comment); err != nil {
			return fmt.Errorf("failed to post inline comment: %w", err)
		}
	}
	return nil
}

func (c *client) postInlineComment(event types.PullRequestEvent, comment types.InlineComment) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d/comments",
		event.Repository.FullName, event.PullRequest.Number)

	payload := map[string]interface{}{
		"body":     fmt.Sprintf("%s\n\nReasoning: %s", comment.Suggestion, comment.Reasoning),
		"path":     comment.File,
		"line":     comment.Line,
		"position": comment.Line,
	}

	return c.postToGitHub(url, payload)
}

func (c *client) postToGitHub(url string, payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
