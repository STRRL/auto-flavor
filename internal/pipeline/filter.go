package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/strrl/auto-flavor/internal/ai"
	"github.com/strrl/auto-flavor/internal/parser"
)

type Candidate struct {
	Content   string
	Timestamp time.Time
	SessionID string
}

type Filter struct {
	client *ai.Client
}

func NewFilter(client *ai.Client) *Filter {
	return &Filter{client: client}
}

type filterResponse struct {
	HasPreference bool    `json:"has_preference"`
	Confidence    float64 `json:"confidence"`
}

func (f *Filter) Filter(ctx context.Context, entries []*parser.ParsedEntry) ([]Candidate, error) {
	var candidates []Candidate

	for _, entry := range entries {
		if entry.Type != "user" {
			continue
		}

		content := strings.TrimSpace(entry.UserContent)
		if content == "" {
			continue
		}

		hasPreference, err := f.checkPreference(ctx, content)
		if err != nil {
			continue
		}

		if hasPreference {
			candidates = append(candidates, Candidate{
				Content:   content,
				Timestamp: entry.Timestamp,
				SessionID: entry.SessionID,
			})
		}
	}

	return candidates, nil
}

func (f *Filter) checkPreference(ctx context.Context, content string) (bool, error) {
	systemPrompt := `You detect if text contains a developer preference or coding rule.

A preference is when someone expresses:
- What they want or don't want (e.g., "don't use semicolons", "use pnpm")
- Coding style rules (e.g., "comments in English", "camelCase for functions")
- Tool preferences (e.g., "use vitest for testing")
- Prohibitions (e.g., "never use any", "no end-of-line comments")
- Communication preferences (e.g., "reply in Chinese", "be concise")

Output JSON only: {"has_preference": true/false, "confidence": 0.0-1.0}`

	userPrompt := fmt.Sprintf("Does this text contain a developer preference?\n\n%s", truncateForPrompt(content, 500))

	resp, err := f.client.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return false, err
	}

	parsed, err := parseFilterResponse(resp)
	if err != nil {
		return false, err
	}

	return parsed.HasPreference && parsed.Confidence >= 0.6, nil
}

func parseFilterResponse(content string) (*filterResponse, error) {
	jsonStr := extractJSON(content)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found")
	}

	var resp filterResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func truncateForPrompt(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
