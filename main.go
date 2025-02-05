package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/crazywolf132/repo-ranger/pkg/api"
	"github.com/crazywolf132/repo-ranger/pkg/diff"
	"github.com/crazywolf132/repo-ranger/pkg/github"
	"github.com/crazywolf132/repo-ranger/pkg/types"
)

const (
	maxChunkSize = 10000 // maximum characters per diff chunk
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
	// Configure logging
	log.SetFormatter(&log.JSONFormatter{})
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		if parsedLevel, err := log.ParseLevel(level); err == nil {
			log.SetLevel(parsedLevel)
		}
	}

	// Get configuration from environment
	apiURL := os.Getenv("INPUT_API_URL")
	apiKey := os.Getenv("INPUT_API_KEY")
	model := os.Getenv("INPUT_MODEL")
	diffCommand := os.Getenv("INPUT_DIFF_COMMAND")
	diffTimeoutSec := getEnvAsInt("INPUT_DIFF_TIMEOUT", 30)
	apiTimeoutSec := getEnvAsInt("INPUT_API_TIMEOUT", 30)
	postPRComment := getEnvAsBool("INPUT_POST_PR_COMMENT", true)
	useChecks := getEnvAsBool("INPUT_USE_CHECKS", false)
	inlineComments := getEnvAsBool("INPUT_INLINE_COMMENTS", false)
	githubToken := os.Getenv("INPUT_GITHUB_TOKEN")
	temperature := getEnvFloat("INPUT_TEMPERATURE", 0.7)
	maxTokens := getEnvInt("INPUT_MAX_TOKENS", 2000)

	// Validate required inputs
	if apiURL == "" || apiKey == "" || model == "" {
		log.WithFields(log.Fields{
			"apiURL": apiURL != "",
			"apiKey": apiKey != "",
			"model":  model != "",
		}).Fatal("Missing required inputs")
		os.Exit(1)
	}

	// Initialize clients
	apiClient := api.NewClient(apiURL, apiKey,
		api.WithRetry(2, 3*time.Second),
		api.WithTemperature(temperature),
		api.WithMaxTokens(maxTokens),
	)
	diffRunner := diff.NewRunner()
	githubClient := github.NewClient(githubToken, nil)

	// Get diff
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(diffTimeoutSec)*time.Second)
	defer cancel()

	log.WithFields(log.Fields{
		"command": diffCommand,
		"timeout": diffTimeoutSec,
	}).Info("Executing diff command")

	diffOutput, err := diffRunner.Run(ctx, diffCommand)
	if err != nil {
		log.WithError(err).Fatal("Failed to execute diff command")
	}

	trimmedDiff := strings.TrimSpace(diffOutput)
	if trimmedDiff == "" {
		log.Info("No code changes detected")
		os.Exit(0)
	}

	// Process the diff
	var finalReview string
	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(apiTimeoutSec)*time.Second)
	defer cancel()

	if len(trimmedDiff) <= maxChunkSize {
		log.WithField("diffSize", len(trimmedDiff)).Debug("Diff size is within limits")
		finalReview, err = apiClient.Review(ctx, model, buildDetailedPrompt(trimmedDiff))
		if err != nil {
			log.WithError(err).Fatal("Failed during API call")
		}
	} else {
		log.WithField("diffSize", len(trimmedDiff)).Info("Large diff detected; performing multi-step review")
		
		chunks := diffRunner.SplitIntoChunks(trimmedDiff, maxChunkSize)
		var reviews []string

		for i, chunk := range chunks {
			log.WithFields(log.Fields{
				"chunk": i + 1,
				"total": len(chunks),
				"size":  len(chunk),
			}).Info("Reviewing chunk")
			
			review, err := apiClient.Review(ctx, model, buildDetailedPrompt(chunk))
			if err != nil {
				log.WithFields(log.Fields{
					"chunk": i + 1,
					"error": err,
				}).Fatal("Failed during detailed review")
			}
			reviews = append(reviews, review)
		}

		finalReview = strings.Join(reviews, "\n\n")
	}

	log.Debug("Review output generated successfully")

	// Handle GitHub integration
	if prEvent, err := parsePullRequestEvent(); err == nil && prEvent.PullRequest.Number > 0 {
		if postPRComment {
			if err := githubClient.PostPRComment(prEvent, finalReview); err != nil {
				log.WithError(err).Error("Failed to post PR comment")
			} else {
				log.Info("PR comment posted successfully")
			}
		}

		if useChecks {
			if err := githubClient.CreateCheckRun(finalReview); err != nil {
				log.WithError(err).Error("Failed to create GitHub Check Run")
			} else {
				log.Info("GitHub Check Run created successfully")
			}
		}

		if inlineComments {
			comments := parseInlineComments(finalReview)
			if len(comments) > 0 {
				if err := githubClient.PostInlineComments(prEvent, comments); err != nil {
					log.WithError(err).Error("Failed to post inline comments")
				} else {
					log.WithField("count", len(comments)).Info("Inline comments posted successfully")
				}
			} else {
				log.Debug("No inline comments found in the aggregated review")
			}
		}
	} else {
		log.WithError(err).Debug("No valid pull request event detected")
	}
}

func getEnvAsInt(name string, defaultVal int) int {
	if v := os.Getenv(name); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvAsBool(name string, defaultVal bool) bool {
	if v := os.Getenv(name); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			return parsed
		}
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return defaultVal
}

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

func parsePullRequestEvent() (types.PullRequestEvent, error) {
	var event types.PullRequestEvent
	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventPath == "" {
		return event, fmt.Errorf("GITHUB_EVENT_PATH not set")
	}

	data, err := os.ReadFile(eventPath)
	if err != nil {
		return event, fmt.Errorf("failed to read event file: %w", err)
	}

	if err := json.Unmarshal(data, &event); err != nil {
		return event, fmt.Errorf("failed to parse event data: %w", err)
	}

	return event, nil
}

func parseInlineComments(review string) []types.InlineComment {
	var comments []types.InlineComment
	lines := strings.Split(review, "\n")
	var current *types.InlineComment

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "InlineComment:"):
			if current != nil {
				comments = append(comments, *current)
			}
			current = &types.InlineComment{}
		case strings.HasPrefix(line, "File: ") && current != nil:
			current.File = strings.TrimPrefix(line, "File: ")
		case strings.HasPrefix(line, "Line: ") && current != nil:
			lineStr := strings.TrimPrefix(line, "Line: ")
			if line, err := strconv.Atoi(lineStr); err == nil {
				current.Line = line
			}
		case strings.HasPrefix(line, "Code Suggestion: ") && current != nil:
			current.Suggestion = strings.TrimPrefix(line, "Code Suggestion: ")
		case strings.HasPrefix(line, "Reasoning: ") && current != nil:
			current.Reasoning = strings.TrimPrefix(line, "Reasoning: ")
		}
	}

	if current != nil {
		comments = append(comments, *current)
	}

	return comments
}
