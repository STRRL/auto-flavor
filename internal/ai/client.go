package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const defaultBaseURL = "https://openrouter.ai/api/v1/chat/completions"

var errMissingAPIKey = fmt.Errorf("OPENROUTER_API_KEY is required")

type Client struct {
	apiKey      string
	baseURL     string
	httpClient  *http.Client
	model       string
	temperature float64
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewClient(cfg Config) (*Client, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return nil, errMissingAPIKey
	}

	baseURL := os.Getenv("OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 45 * time.Second
	}

	temperature := cfg.Temperature
	if temperature == 0 {
		temperature = 0.1
	}

	return &Client{
		apiKey:      apiKey,
		baseURL:     baseURL,
		httpClient:  &http.Client{Timeout: timeout},
		model:       cfg.Model,
		temperature: temperature,
	}, nil
}

func (c *Client) Chat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	payload := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: c.temperature,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("openrouter error: status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed chatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if parsed.Error != nil {
		return "", fmt.Errorf("openrouter error: %s", parsed.Error.Message)
	}

	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openrouter returned no choices")
	}

	return parsed.Choices[0].Message.Content, nil
}
