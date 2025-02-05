package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/crazywolf132/repo-ranger/pkg/types"
)

const (
	defaultTemperature = 0.7
	defaultMaxTokens   = 2000
	openAIEndpoint    = "https://api.openai.com/v1/chat/completions"
)

// Client represents an API client for the code review service.
type Client interface {
	Review(ctx context.Context, model, prompt string) (string, error)
}

// HTTPClient represents the interface for making HTTP requests.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type client struct {
	baseURL     string
	apiKey      string
	httpClient  HTTPClient
	retryCount  int
	retryDelay  time.Duration
	temperature float64
	maxTokens   int
}

// ClientOption is a function that configures a client.
type ClientOption func(*client)

// WithRetry sets the retry configuration for the client.
func WithRetry(count int, delay time.Duration) ClientOption {
	return func(c *client) {
		c.retryCount = count
		c.retryDelay = delay
	}
}

// WithHTTPClient sets the HTTP client for the API client.
func WithHTTPClient(httpClient HTTPClient) ClientOption {
	return func(c *client) {
		c.httpClient = httpClient
	}
}

// WithTemperature sets the temperature for the OpenAI API.
func WithTemperature(temperature float64) ClientOption {
	return func(c *client) {
		c.temperature = temperature
	}
}

// WithMaxTokens sets the max tokens for the OpenAI API.
func WithMaxTokens(maxTokens int) ClientOption {
	return func(c *client) {
		c.maxTokens = maxTokens
	}
}

// NewClient creates a new API client.
func NewClient(baseURL, apiKey string, opts ...ClientOption) Client {
	c := &client{
		baseURL:     baseURL,
		apiKey:      apiKey,
		httpClient:  &http.Client{},
		retryCount:  2,
		retryDelay:  3 * time.Second,
		temperature: defaultTemperature,
		maxTokens:   defaultMaxTokens,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Review sends a review request to the API.
func (c *client) Review(ctx context.Context, model, prompt string) (string, error) {
	var lastErr error
	for i := 0; i <= c.retryCount; i++ {
		if i > 0 {
			log.WithFields(log.Fields{
				"attempt": i,
				"delay":   c.retryDelay,
			}).Debug("Retrying API call")
			time.Sleep(c.retryDelay)
		}

		review, err := c.makeRequest(ctx, model, prompt)
		if err == nil {
			return review, nil
		}
		lastErr = err
		log.WithFields(log.Fields{
			"attempt": i + 1,
			"error":   err,
		}).Warn("API call failed")
	}
	return "", fmt.Errorf("API call failed after %d attempts: %w", c.retryCount+1, lastErr)
}

func (c *client) makeRequest(ctx context.Context, model, prompt string) (string, error) {
	messages := []types.OpenAIMessage{
		{
			Role:    "system",
			Content: "You are an expert code reviewer. Analyze the code changes and provide detailed, actionable feedback.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	payload := types.OpenAIRequest{
		Model:       model,
		Messages:    messages,
		Temperature: c.temperature,
		MaxTokens:   c.maxTokens,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Use OpenAI endpoint if baseURL is not specified
	endpoint := c.baseURL
	if endpoint == "" {
		endpoint = openAIEndpoint
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned non-200 status code %d: %s", resp.StatusCode, string(body))
	}

	var apiResp types.OpenAIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned in API response")
	}

	review := apiResp.Choices[0].Message.Content
	return review, nil
}
