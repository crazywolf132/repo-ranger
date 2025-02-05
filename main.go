package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// Payload is the JSON structure sent to the review API.
type Payload struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// APIResponse represents the expected structure of the API response.
type APIResponse struct {
	Review string `json:"review"`
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

const (
	apiCallMaxRetries = 2
	apiCallRetryDelay = 3 * time.Second
	maxChunkSize      = 10000 // maximum characters per diff chunk
)

var (
	// ErrMissingInputs is returned when required inputs are not provided
	ErrMissingInputs = errors.New("missing required inputs")
	// ErrNoDiffOutput is returned when no code changes are detected
	ErrNoDiffOutput = errors.New("no code changes detected")
	// ErrAPICall is returned when there's an error calling the review API
	ErrAPICall = errors.New("error calling review API")
	// ErrGitHubAPI is returned when there's an error interacting with GitHub API
	ErrGitHubAPI = errors.New("error interacting with GitHub API")
)

func init() {
	// Configure logrus
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	
	// Set log level based on environment variable, default to info
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)
}

func main() {
	// Retrieve inputs.
	apiURL := os.Getenv("INPUT_API_URL")
	apiKey := os.Getenv("INPUT_API_KEY")
	model := os.Getenv("INPUT_MODEL")
	diffCommand := os.Getenv("INPUT_DIFF_COMMAND")
	if diffCommand == "" {
		diffCommand = "git diff HEAD~1 HEAD"
	}
	diffTimeoutSec := getEnvAsInt("INPUT_DIFF_TIMEOUT", 30)
	apiTimeoutSec := getEnvAsInt("INPUT_API_TIMEOUT", 30)
	postPRComment := getEnvAsBool("INPUT_POST_PR_COMMENT", true)
	useChecks := getEnvAsBool("INPUT_USE_CHECKS", false)
	inlineComments := getEnvAsBool("INPUT_INLINE_COMMENTS", false)
	githubToken := os.Getenv("INPUT_GITHUB_TOKEN")

	// Validate required inputs.
	if apiURL == "" || apiKey == "" || model == "" {
		log.WithFields(log.Fields{
			"apiURL": apiURL != "",
			"apiKey": apiKey != "",
			"model":  model != "",
		}).Fatal("Missing required inputs")
		os.Exit(1)
	}

	log.WithFields(log.Fields{
		"command": diffCommand,
		"timeout": diffTimeoutSec,
	}).Info("Executing diff command")

	diffOutput, err := runDiff(diffCommand, time.Duration(diffTimeoutSec)*time.Second)
	if err != nil {
		log.WithError(err).Fatal("Failed to execute diff command")
	}

	trimmedDiff := strings.TrimSpace(diffOutput)
	if trimmedDiff == "" {
		log.Info("No code changes detected")
		os.Exit(0)
	}

	var finalReview string
	if len(trimmedDiff) <= maxChunkSize {
		log.WithField("diffSize", len(trimmedDiff)).Debug("Diff size is within limits")
		detailedPrompt := buildDetailedPrompt(trimmedDiff)
		payload := Payload{Model: model, Prompt: detailedPrompt}
		finalReview, err = callAPIWithRetries(apiURL, apiKey, payload, time.Duration(apiTimeoutSec)*time.Second)
		if err != nil {
			log.WithError(err).Fatal("Failed during API call")
		}
	} else {
		log.WithField("diffSize", len(trimmedDiff)).Info("Large diff detected; performing multi-step review")
		
		summaryInput := trimmedDiff
		if len(trimmedDiff) > maxChunkSize {
			summaryInput = trimmedDiff[:maxChunkSize]
		}
		
		summary, err := getSummary(summaryInput, model, apiURL, apiKey, time.Duration(apiTimeoutSec)*time.Second)
		if err != nil {
			log.WithError(err).Fatal("Failed to obtain summary")
		}
		log.Debug("High-level summary obtained")

		chunks := splitIntoChunks(trimmedDiff, maxChunkSize)
		var detailedReviews []string
		for i, chunk := range chunks {
			log.WithFields(log.Fields{
				"chunk": i + 1,
				"total": len(chunks),
				"size":  len(chunk),
			}).Info("Reviewing chunk")
			
			detail, err := getDetailedReview(chunk, model, apiURL, apiKey, time.Duration(apiTimeoutSec)*time.Second)
			if err != nil {
				log.WithFields(log.Fields{
					"chunk": i + 1,
					"error": err,
				}).Fatal("Failed during detailed review")
			}
			detailedReviews = append(detailedReviews, detail)
		}
		finalReview = fmt.Sprintf("### High-Level Summary\n%s\n\n### Detailed Review\n%s", summary, strings.Join(detailedReviews, "\n\n"))
	}

	log.Debug("Review output generated successfully")

	formattedReview := formatReviewForPR(finalReview)

	if outputPath := os.Getenv("GITHUB_OUTPUT"); outputPath != "" {
		if err := appendOutput(outputPath, "review", formattedReview); err != nil {
			log.WithError(err).Error("Failed to write GitHub Action output")
		}
	}

	if postPRComment && githubToken != "" {
		if prEvent, err := parsePullRequestEvent(); err == nil && prEvent.PullRequest.Number > 0 {
			if err := postPRCommentFunc(githubToken, prEvent, formattedReview); err != nil {
				log.WithFields(log.Fields{
					"pr":    prEvent.PullRequest.Number,
					"repo":  prEvent.Repository.FullName,
					"error": err,
				}).Error("Failed to post PR comment")
			} else {
				log.Info("PR comment posted successfully")
			}
		} else {
			log.WithError(err).Debug("No valid pull request event detected")
		}
	} else {
		log.Debug("PR comment posting is disabled or GitHub token not provided")
	}

	if useChecks && githubToken != "" {
		if err := createCheckRun(githubToken, formattedReview); err != nil {
			log.WithError(err).Error("Failed to create GitHub Check Run")
		} else {
			log.Info("GitHub Check Run created successfully")
		}
	} else {
		log.Debug("GitHub Check Run creation is disabled or GitHub token not provided")
	}

	if inlineComments && githubToken != "" {
		if prEvent, err := parsePullRequestEvent(); err == nil && prEvent.PullRequest.Number > 0 {
			inlineCommentsList := parseInlineComments(finalReview)
			if len(inlineCommentsList) > 0 {
				if err := postInlineComments(githubToken, prEvent, inlineCommentsList); err != nil {
					log.WithFields(log.Fields{
						"pr":       prEvent.PullRequest.Number,
						"repo":     prEvent.Repository.FullName,
						"comments": len(inlineCommentsList),
						"error":    err,
					}).Error("Failed to post inline comments")
				} else {
					log.WithField("count", len(inlineCommentsList)).Info("Inline comments posted successfully")
				}
			} else {
				log.Debug("No inline comments found in the aggregated review")
			}
		} else {
			log.WithError(err).Debug("No valid pull request event detected")
		}
	} else {
		log.Debug("Inline comment posting is disabled or GitHub token not provided")
	}
}

// buildDetailedPrompt constructs the prompt for a detailed, line-by-line review.
func buildDetailedPrompt(diff string) string {
	var b strings.Builder
	b.WriteString("Perform a detailed, line-by-line review of the following code changes. ")
	b.WriteString("For each changed line, output your review in the following format (each on a separate line):\n")
	b.WriteString("InlineComment:\n")
	b.WriteString("File: <file path>\n")
	b.WriteString("Line: <line number>\n")
	b.WriteString("Code Suggestion: <your suggested code change>\n")
	b.WriteString("Reasoning: <explanation for the suggestion>\n")
	b.WriteString("\nThen, provide an aggregated summary at the top.\n\n")
	b.WriteString(diff)
	return b.String()
}

// getEnvAsInt reads an environment variable as an integer, or returns a default.
func getEnvAsInt(name string, defaultVal int) int {
	if v := os.Getenv(name); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

// getEnvAsBool reads an environment variable as a boolean, or returns a default.
func getEnvAsBool(name string, defaultVal bool) bool {
	if v := os.Getenv(name); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultVal
}

// runDiff executes the specified command with a timeout.
func runDiff(commandStr string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", commandStr)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("diff command failed with stderr: %s: %w", exitErr.Stderr, err)
		}
		return "", fmt.Errorf("failed to execute diff command: %w", err)
	}

	return string(output), nil
}

// callAPIWithRetries calls the review API with retry logic.
func callAPIWithRetries(apiURL, apiKey string, payload Payload, timeout time.Duration) (string, error) {
	var lastErr error
	for i := 0; i <= apiCallMaxRetries; i++ {
		if i > 0 {
			log.WithFields(log.Fields{
				"attempt": i,
				"delay":   apiCallRetryDelay,
			}).Debug("Retrying API call")
			time.Sleep(apiCallRetryDelay)
		}

		review, err := callAPI(apiURL, apiKey, payload, timeout)
		if err == nil {
			return review, nil
		}
		lastErr = err
		log.WithFields(log.Fields{
			"attempt": i + 1,
			"error":   err,
		}).Warn("API call failed")
	}
	return "", fmt.Errorf("%w: %v", ErrAPICall, lastErr)
}

// callAPI sends the payload to the review API and returns the review.
func callAPI(apiURL, apiKey string, payload Payload, timeout time.Duration) (string, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
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

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return apiResp.Review, nil
}

// getSummary obtains a high-level summary of the diff.
func getSummary(diff, model, apiURL, apiKey string, timeout time.Duration) (string, error) {
	prompt := "Provide a high-level summary of the following code changes, including overall impact, potential issues, and recommendations:\n\n" + diff
	payload := Payload{Model: model, Prompt: prompt}
	return callAPIWithRetries(apiURL, apiKey, payload, timeout)
}

// getDetailedReview obtains a detailed, line-by-line review for a diff chunk.
func getDetailedReview(diffChunk, model, apiURL, apiKey string, timeout time.Duration) (string, error) {
	// The prompt instructs the AI to output inline review comments in a structured format.
	prompt := buildDetailedPrompt(diffChunk)
	payload := Payload{Model: model, Prompt: prompt}
	return callAPIWithRetries(apiURL, apiKey, payload, timeout)
}

// splitIntoChunks splits the diff into chunks not exceeding maxChunkSize.
func splitIntoChunks(diff string, maxChunkSize int) []string {
	lines := strings.Split(diff, "\n")
	var chunks []string
	var currentChunk strings.Builder
	for _, line := range lines {
		if currentChunk.Len()+len(line)+1 > maxChunkSize {
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
		}
		currentChunk.WriteString(line)
		currentChunk.WriteString("\n")
	}
	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}
	return chunks
}

// appendOutput writes the aggregated review to the GitHub Actions output file.
func appendOutput(path, name, value string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("error opening GITHUB_OUTPUT file: %w", err)
	}
	defer f.Close()
	output := fmt.Sprintf("%s<<EOF\n%s\nEOF\n", name, value)
	if _, err := f.WriteString(output); err != nil {
		return fmt.Errorf("error writing to GITHUB_OUTPUT file: %w", err)
	}
	return nil
}

// parsePullRequestEvent reads and parses the GitHub event payload.
func parsePullRequestEvent() (PullRequestEvent, error) {
	var prEvent PullRequestEvent
	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventPath == "" {
		return prEvent, fmt.Errorf("GITHUB_EVENT_PATH not set")
	}
	data, err := os.ReadFile(eventPath)
	if err != nil {
		return prEvent, fmt.Errorf("error reading GITHUB_EVENT_PATH: %v", err)
	}
	if err := json.Unmarshal(data, &prEvent); err != nil {
		return prEvent, fmt.Errorf("error parsing GitHub event payload: %v", err)
	}
	return prEvent, nil
}

// formatReviewForPR formats the aggregated review to be more developer-friendly,
// wrapping code suggestions in GitHub's suggestion markdown and bolding reasoning.
func formatReviewForPR(review string) string {
	var builder strings.Builder
	builder.WriteString("## Repo Ranger Code Review\n\n")
	lines := strings.Split(review, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Code Suggestion:") {
			suggestion := strings.TrimSpace(strings.TrimPrefix(line, "Code Suggestion:"))
			builder.WriteString("**Code Suggestion:**\n```suggestion\n" + suggestion + "\n```\n\n")
		} else if strings.HasPrefix(line, "Reasoning:") {
			reasoning := strings.TrimSpace(strings.TrimPrefix(line, "Reasoning:"))
			builder.WriteString("**Reasoning:** " + reasoning + "\n\n")
		} else {
			builder.WriteString(line + "\n")
		}
	}
	return builder.String()
}

// postPRCommentFunc posts the aggregated review as a PR comment.
func postPRCommentFunc(token string, event PullRequestEvent, review string) error {
	repoFullName := event.Repository.FullName
	if repoFullName == "" {
		return fmt.Errorf("repository full name not found in event payload")
	}
	prNumber := event.PullRequest.Number
	if prNumber == 0 {
		return fmt.Errorf("pull request number not found in event payload")
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/comments", repoFullName, prNumber)
	commentBody := map[string]string{"body": review}
	commentBytes, err := json.Marshal(commentBody)
	if err != nil {
		return fmt.Errorf("error marshalling comment body: %v", err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(commentBytes))
	if err != nil {
		return fmt.Errorf("error creating HTTP request for PR comment: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("User-Agent", "Repo-Ranger-Action/2.0")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error posting PR comment: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to post PR comment, status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// createCheckRun creates a GitHub Check Run with the review output.
func createCheckRun(token, review string) error {
	repo := os.Getenv("GITHUB_REPOSITORY")
	headSHA := os.Getenv("GITHUB_SHA")
	if repo == "" || headSHA == "" {
		return fmt.Errorf("GITHUB_REPOSITORY or GITHUB_SHA not set")
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/check-runs", repo)
	payload := map[string]interface{}{
		"name":       "Repo Ranger Code Review",
		"head_sha":   headSHA,
		"status":     "completed",
		"conclusion": "success",
		"output": map[string]string{
			"title":   "Repo Ranger Code Review",
			"summary": "The following is the aggregated review output from Repo Ranger:",
			"text":    review,
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshalling check run payload: %v", err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("error creating check run HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Repo-Ranger-Action/2.0")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error creating check run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create check run, status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// parseInlineComments scans the aggregated review text for inline comment markers
// and returns a slice of InlineComment structs.
func parseInlineComments(review string) []InlineComment {
	var comments []InlineComment
	lines := strings.Split(review, "\n")
	var current *InlineComment
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "InlineComment:" {
			current = &InlineComment{}
		} else if current != nil {
			if strings.HasPrefix(line, "File:") {
				current.File = strings.TrimSpace(strings.TrimPrefix(line, "File:"))
			} else if strings.HasPrefix(line, "Line:") {
				if l, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Line:"))); err == nil {
					current.Line = l
				}
			} else if strings.HasPrefix(line, "Code Suggestion:") {
				current.Suggestion = strings.TrimSpace(strings.TrimPrefix(line, "Code Suggestion:"))
			} else if strings.HasPrefix(line, "Reasoning:") {
				current.Reasoning = strings.TrimSpace(strings.TrimPrefix(line, "Reasoning:"))
				// End of an inline comment block.
				comments = append(comments, *current)
				current = nil
			}
		}
	}
	return comments
}

// postInlineComment posts a single inline review comment to the PR.
func postInlineComment(token string, event PullRequestEvent, comment InlineComment) error {
	repoFullName := event.Repository.FullName
	prNumber := event.PullRequest.Number
	commitID := os.Getenv("GITHUB_SHA")
	if repoFullName == "" || prNumber == 0 || commitID == "" {
		return fmt.Errorf("required PR details not found in environment")
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d/comments", repoFullName, prNumber)
	bodyText := fmt.Sprintf(
		"**Code Suggestion:**\n```suggestion\n%s\n```\n\n**Reasoning:** %s",
		comment.Suggestion,
		comment.Reasoning,
	)
	payload := map[string]interface{}{
		"body":      bodyText,
		"commit_id": commitID,
		"path":      comment.File,
		"line":      comment.Line,
		"side":      "RIGHT",
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshalling inline comment payload: %v", err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("error creating HTTP request for inline comment: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("User-Agent", "Repo-Ranger-Action/2.0")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error posting inline comment: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to post inline comment, status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// postInlineComments iterates over inline comments and posts them.
func postInlineComments(token string, event PullRequestEvent, comments []InlineComment) error {
	for _, c := range comments {
		if err := postInlineComment(token, event, c); err != nil {
			return err
		}
	}
	return nil
}
