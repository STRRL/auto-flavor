package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/strrl/auto-flavor/internal/ai"
	"github.com/strrl/auto-flavor/internal/signals"
)

type Extractor struct {
	client *ai.Client
}

func NewExtractor(client *ai.Client) *Extractor {
	return &Extractor{client: client}
}

type extractResponse struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Strength    string `json:"strength"`
}

func (e *Extractor) Extract(ctx context.Context, classified []ClassifiedCandidate) ([]signals.Signal, error) {
	var results []signals.Signal

	for _, cand := range classified {
		sig, err := e.extractOne(ctx, cand)
		if err != nil {
			continue
		}
		results = append(results, *sig)
	}

	return results, nil
}

func (e *Extractor) extractOne(ctx context.Context, cand ClassifiedCandidate) (*signals.Signal, error) {
	systemPrompt := buildExtractorSystemPrompt(cand.Category)
	userPrompt := fmt.Sprintf("Extract the preference from this text:\n\n%s", cand.Content)

	content, err := e.client.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	resp, err := parseExtractResponse(content)
	if err != nil {
		return nil, err
	}

	if resp.Title == "" || resp.Description == "" {
		return nil, fmt.Errorf("empty title or description")
	}

	strength := signals.StrengthInferred
	if resp.Strength == "explicit" {
		strength = signals.StrengthExplicit
	}

	return &signals.Signal{
		Category:    cand.Category,
		Title:       resp.Title,
		Description: resp.Description,
		Strength:    strength,
		Timestamp:   cand.Timestamp,
		Context:     truncate(cand.Content, 200),
	}, nil
}

func buildExtractorSystemPrompt(category signals.Category) string {
	desc := signals.ValidCategories[category]

	return fmt.Sprintf(`You extract developer preferences from text.

Category: %s (%s)

Output JSON only:
{
  "title": "short-kebab-case-title",
  "description": "Clear description of the preference rule",
  "strength": "explicit" or "inferred"
}

- title: A short identifier in kebab-case (e.g., "no-end-of-line-comments", "use-pnpm")
- description: A clear, actionable rule (e.g., "Do not use end-of-line comments", "Use pnpm as package manager")
- strength: "explicit" if the user directly stated the preference, "inferred" if deduced from context

Keep the description concise but complete.
`, category, desc)
}

func parseExtractResponse(content string) (*extractResponse, error) {
	jsonStr := extractJSON(content)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	var resp extractResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
