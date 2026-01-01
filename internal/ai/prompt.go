package ai

import (
	"encoding/json"
	"fmt"
	"time"
)

type promptEntry struct {
	Role      string       `json:"role"`
	Timestamp string       `json:"timestamp"`
	Content   string       `json:"content,omitempty"`
	Tools     []promptTool `json:"tools,omitempty"`
}

type promptTool struct {
	Name  string `json:"name"`
	Input string `json:"input"`
}

func BuildPrompt(entries []Entry) (string, string, error) {
	serialized := make([]promptEntry, 0, len(entries))
	for _, entry := range entries {
		var tools []promptTool
		if len(entry.Tools) > 0 {
			tools = make([]promptTool, 0, len(entry.Tools))
			for _, tool := range entry.Tools {
				tools = append(tools, promptTool{
					Name:  tool.Name,
					Input: tool.Input,
				})
			}
		}

		timestamp := ""
		if !entry.Timestamp.IsZero() {
			timestamp = entry.Timestamp.UTC().Format(time.RFC3339)
		}

		serialized = append(serialized, promptEntry{
			Role:      entry.Role,
			Timestamp: timestamp,
			Content:   entry.Content,
			Tools:     tools,
		})
	}

	payload, err := json.Marshal(serialized)
	if err != nil {
		return "", "", fmt.Errorf("failed to serialize prompt entries: %w", err)
	}

	systemPrompt := "You extract developer preference signals from chat logs. Return only JSON."

	userPrompt := fmt.Sprintf(`Input entries (JSON array):
%s

Rules:
- Extract signals of type approval, correction, style, or stack.
- Use only the provided entries.
- Output JSON only, with this schema:
  {"signals":[{"type":"approval|correction|style|stack","group":"stack|style|workflow|tooling|quality|communication|testing|misc","category":"...","key":"...","value":"...","strength":"weak|moderate|strong|explicit","timestamp":"RFC3339","context":"short evidence"}]}
- Use the timestamp of the entry that expresses the signal.
- For stack signals, infer languages/tools/frameworks from file paths, commands, or explicit mentions.
- Choose a group from the allowed list; if unsure use "misc".
- Keep context <= 120 characters.
- If no signals, return {"signals":[]}.
`, string(payload))

	return systemPrompt, userPrompt, nil
}
