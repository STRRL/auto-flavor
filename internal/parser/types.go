package parser

import (
	"encoding/json"
	"time"
)

type ChatEntry struct {
	Type       string          `json:"type"`
	Message    json.RawMessage `json:"message"`
	Timestamp  time.Time       `json:"timestamp"`
	SessionID  string          `json:"sessionId"`
	UUID       string          `json:"uuid"`
	ParentUUID string          `json:"parentUuid"`
	CWD        string          `json:"cwd"`
}

type UserMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AssistantMessage struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type EditToolInput struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

type WriteToolInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type BashToolInput struct {
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
}

type ToolResultContent struct {
	ToolUseID string `json:"tool_use_id"`
	Type      string `json:"type"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error"`
}

type ParsedEntry struct {
	Type       string
	Timestamp  time.Time
	SessionID  string
	UUID       string
	ParentUUID string
	CWD        string

	UserContent      string
	AssistantContent []ContentBlock
}

func (e *ChatEntry) Parse() (*ParsedEntry, error) {
	parsed := &ParsedEntry{
		Type:       e.Type,
		Timestamp:  e.Timestamp,
		SessionID:  e.SessionID,
		UUID:       e.UUID,
		ParentUUID: e.ParentUUID,
		CWD:        e.CWD,
	}

	if e.Type == "user" {
		var userMsg UserMessage
		if err := json.Unmarshal(e.Message, &userMsg); err != nil {
			parsed.UserContent = string(e.Message)
		} else {
			parsed.UserContent = userMsg.Content
		}
	} else if e.Type == "assistant" {
		var assistantMsg AssistantMessage
		if err := json.Unmarshal(e.Message, &assistantMsg); err == nil {
			parsed.AssistantContent = assistantMsg.Content
		}
	}

	return parsed, nil
}

func (p *ParsedEntry) GetToolUses() []ContentBlock {
	var tools []ContentBlock
	for _, block := range p.AssistantContent {
		if block.Type == "tool_use" {
			tools = append(tools, block)
		}
	}
	return tools
}

func (p *ParsedEntry) GetTextContent() string {
	for _, block := range p.AssistantContent {
		if block.Type == "text" {
			return block.Text
		}
	}
	return ""
}
