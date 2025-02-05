package types

// ReviewPayload is the JSON structure sent to the review API.
type ReviewPayload struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// ReviewResponse represents the expected structure of the API response.
type ReviewResponse struct {
	Review string `json:"review"`
}

// OpenAIMessage represents a message in the OpenAI chat format
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIRequest represents the request structure for OpenAI's chat completion API
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
}

// OpenAIResponse represents the response structure from OpenAI's chat completion API
type OpenAIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a completion choice in the OpenAI response
type Choice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// Usage represents token usage in the OpenAI response
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// PullRequestEvent is used to parse the GitHub event payload.
type PullRequestEvent struct {
	PullRequest struct {
		Number int `json:"number"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"` // e.g., "owner/repo"
	} `json:"repository"`
}

// InlineComment represents a structured inline review comment.
type InlineComment struct {
	File       string
	Line       int
	Suggestion string
	Reasoning  string
}
