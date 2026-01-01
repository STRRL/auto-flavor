package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/strrl/auto-flavor/internal/parser"
	"github.com/strrl/auto-flavor/internal/signals"
)

type Detector struct {
	client    *Client
	batchSize int
}

var allowedGroups = map[string]struct{}{
	"stack":         {},
	"style":         {},
	"workflow":      {},
	"tooling":       {},
	"quality":       {},
	"communication": {},
	"testing":       {},
	"misc":          {},
}

func NewDetector(cfg Config) (*Detector, error) {
	if cfg.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 30
	}

	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return &Detector{
		client:    client,
		batchSize: cfg.BatchSize,
	}, nil
}

func (d *Detector) DetectSignals(entries []*parser.ParsedEntry) ([]signals.Signal, error) {
	aiEntries := toAIEntries(entries)
	if len(aiEntries) == 0 {
		return nil, nil
	}

	var collected []signals.Signal

	for i := 0; i < len(aiEntries); i += d.batchSize {
		end := i + d.batchSize
		if end > len(aiEntries) {
			end = len(aiEntries)
		}

		batch := aiEntries[i:end]
		systemPrompt, userPrompt, err := BuildPrompt(batch)
		if err != nil {
			return nil, err
		}

		content, err := d.client.Chat(context.Background(), systemPrompt, userPrompt)
		if err != nil {
			return nil, err
		}

		parsed, err := parseOutput(content)
		if err != nil {
			return nil, err
		}

		batchSignals := toSignals(parsed)
		collected = append(collected, batchSignals...)
	}

	sort.Slice(collected, func(i, j int) bool {
		return collected[i].Timestamp.Before(collected[j].Timestamp)
	})

	return collected, nil
}

func toAIEntries(entries []*parser.ParsedEntry) []Entry {
	var aiEntries []Entry

	for _, entry := range entries {
		switch entry.Type {
		case "user":
			content := strings.TrimSpace(entry.UserContent)
			if content == "" {
				continue
			}
			aiEntries = append(aiEntries, Entry{
				Role:      "user",
				Timestamp: entry.Timestamp,
				Content:   content,
			})
		case "assistant":
			content := strings.TrimSpace(entry.GetTextContent())
			tools := extractTools(entry)
			if content == "" && len(tools) == 0 {
				continue
			}
			aiEntries = append(aiEntries, Entry{
				Role:      "assistant",
				Timestamp: entry.Timestamp,
				Content:   content,
				Tools:     tools,
			})
		}
	}

	return aiEntries
}

func extractTools(entry *parser.ParsedEntry) []ToolUse {
	tools := entry.GetToolUses()
	if len(tools) == 0 {
		return nil
	}

	result := make([]ToolUse, 0, len(tools))
	for _, tool := range tools {
		input := strings.TrimSpace(string(tool.Input))
		result = append(result, ToolUse{
			Name:  tool.Name,
			Input: input,
		})
	}

	return result
}

func parseOutput(content string) (*Output, error) {
	jsonPayload := extractJSON(content)
	if jsonPayload == "" {
		return nil, fmt.Errorf("no JSON object found in model output")
	}

	var parsed Output
	if err := json.Unmarshal([]byte(jsonPayload), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse model output: %w", err)
	}

	return &parsed, nil
}

func extractJSON(content string) string {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return strings.TrimSpace(content[start : end+1])
}

func toSignals(output *Output) []signals.Signal {
	if output == nil {
		return nil
	}

	var results []signals.Signal
	for _, sig := range output.Signals {
		converted, ok := convertSignal(sig)
		if !ok {
			continue
		}
		results = append(results, converted)
	}

	return results
}

func convertSignal(sig OutputSignal) (signals.Signal, bool) {
	sigType := signals.SignalType(strings.ToLower(strings.TrimSpace(sig.Type)))
	if sigType != signals.SignalApproval && sigType != signals.SignalCorrection && sigType != signals.SignalStyle && sigType != signals.SignalStack {
		return signals.Signal{}, false
	}

	group := normalizeGroup(sig.Group, sigType)
	category := strings.TrimSpace(sig.Category)
	key := strings.TrimSpace(sig.Key)
	value := strings.TrimSpace(sig.Value)
	if category == "" || key == "" || value == "" {
		return signals.Signal{}, false
	}

	timestamp := time.Now()
	if sig.Timestamp != "" {
		if parsed, err := time.Parse(time.RFC3339, sig.Timestamp); err == nil {
			timestamp = parsed
		}
	}

	context := strings.TrimSpace(sig.Context)
	if len(context) > 200 {
		context = context[:200]
	}

	return signals.Signal{
		Type:      sigType,
		Group:     group,
		Category:  category,
		Key:       key,
		Value:     value,
		Strength:  parseStrength(sig.Strength),
		Timestamp: timestamp,
		Context:   context,
	}, true
}

func parseStrength(value string) signals.SignalStrength {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "explicit":
		return signals.StrengthExplicit
	case "strong":
		return signals.StrengthStrong
	case "moderate":
		return signals.StrengthModerate
	case "weak":
		return signals.StrengthWeak
	default:
		return signals.StrengthModerate
	}
}

func normalizeGroup(group string, sigType signals.SignalType) string {
	group = strings.ToLower(strings.TrimSpace(group))
	if group != "" {
		if _, ok := allowedGroups[group]; ok {
			return group
		}
	}

	switch sigType {
	case signals.SignalStack:
		return "stack"
	case signals.SignalStyle:
		return "style"
	case signals.SignalApproval, signals.SignalCorrection:
		return "communication"
	default:
		return "misc"
	}
}
