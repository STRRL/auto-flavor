package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/strrl/auto-flavor/internal/ai"
	"github.com/strrl/auto-flavor/internal/signals"
)

type ClassifiedCandidate struct {
	Candidate
	Category   signals.Category
	Confidence float64
}

type Classifier struct {
	client *ai.Client
}

func NewClassifier(client *ai.Client) *Classifier {
	return &Classifier{client: client}
}

type classifyResponse struct {
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
}

func (c *Classifier) Classify(ctx context.Context, candidates []Candidate) ([]ClassifiedCandidate, error) {
	var results []ClassifiedCandidate

	for _, cand := range candidates {
		resp, err := c.classifyOne(ctx, cand)
		if err != nil {
			continue
		}

		cat := signals.Category(resp.Category)
		if !cat.IsValid() {
			continue
		}

		if resp.Confidence < 0.5 {
			continue
		}

		results = append(results, ClassifiedCandidate{
			Candidate:  cand,
			Category:   cat,
			Confidence: resp.Confidence,
		})
	}

	return results, nil
}

func (c *Classifier) classifyOne(ctx context.Context, cand Candidate) (*classifyResponse, error) {
	systemPrompt := buildClassifierSystemPrompt()
	userPrompt := fmt.Sprintf("Classify this text:\n\n%s", cand.Content)

	content, err := c.client.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	return parseClassifyResponse(content)
}

func buildClassifierSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("You are a classifier. Determine which category a developer preference belongs to.\n\n")
	sb.WriteString("Categories:\n")

	for cat, desc := range signals.ValidCategories {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", cat, desc))
	}

	sb.WriteString("\nOutput JSON only: {\"category\": \"...\", \"confidence\": 0.0-1.0}\n")
	sb.WriteString("If the text does not express a preference, use category \"none\" with low confidence.\n")

	return sb.String()
}

func parseClassifyResponse(content string) (*classifyResponse, error) {
	jsonStr := extractJSON(content)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	var resp classifyResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

func extractJSON(content string) string {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return strings.TrimSpace(content[start : end+1])
}
