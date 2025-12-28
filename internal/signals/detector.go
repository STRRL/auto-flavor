package signals

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/strrl/auto-flavor/internal/parser"
)

type Detector struct {
	approvalPatterns   []*approvalPattern
	correctionPatterns []*correctionPattern
	extensionToLang    map[string]string
	commandToTool      map[string]string
}

type approvalPattern struct {
	Pattern  *regexp.Regexp
	Strength SignalStrength
}

type correctionPattern struct {
	Pattern  *regexp.Regexp
	Strength SignalStrength
	Category string
}

func NewDetector() *Detector {
	return &Detector{
		approvalPatterns: []*approvalPattern{
			{regexp.MustCompile(`(?i)^(good|great|nice|perfect|excellent|awesome|lgtm|looks good)[\s!.]*$`), StrengthStrong},
			{regexp.MustCompile(`(?i)^(thanks|thx|thank you|ty)[\s!.]*$`), StrengthModerate},
			{regexp.MustCompile(`(?i)^(ok|okay|sure|yes|yep|yeah)[\s!.]*$`), StrengthWeak},
			{regexp.MustCompile(`(?i)(good|great|nice) (job|work)`), StrengthStrong},
		},
		correctionPatterns: []*correctionPattern{
			{regexp.MustCompile(`(?i)^no[,.]?\s+(.+)`), StrengthStrong, "rejection"},
			{regexp.MustCompile(`(?i)(don't|do not|shouldn't|should not|never)\s+(.+)`), StrengthExplicit, "prohibition"},
			{regexp.MustCompile(`(?i)use\s+(.+)\s+instead\s+of\s+(.+)`), StrengthExplicit, "preference"},
			{regexp.MustCompile(`(?i)prefer\s+(.+)\s+over\s+(.+)`), StrengthExplicit, "preference"},
			{regexp.MustCompile(`(?i)actually[,.]?\s+(.+)`), StrengthStrong, "correction"},
			{regexp.MustCompile(`(?i)^(fix|change|update|modify)\s+(.+)`), StrengthModerate, "correction"},
			{regexp.MustCompile(`(?i)always\s+(.+)`), StrengthExplicit, "requirement"},
		},
		extensionToLang: map[string]string{
			".go":    "Go",
			".ts":    "TypeScript",
			".tsx":   "TypeScript/React",
			".js":    "JavaScript",
			".jsx":   "JavaScript/React",
			".py":    "Python",
			".rs":    "Rust",
			".java":  "Java",
			".kt":    "Kotlin",
			".rb":    "Ruby",
			".php":   "PHP",
			".cs":    "C#",
			".cpp":   "C++",
			".c":     "C",
			".swift": "Swift",
			".sql":   "SQL",
			".sh":    "Shell",
			".yaml":  "YAML",
			".yml":   "YAML",
			".json":  "JSON",
			".md":    "Markdown",
		},
		commandToTool: map[string]string{
			"npm":    "npm",
			"yarn":   "yarn",
			"pnpm":   "pnpm",
			"go":     "go",
			"cargo":  "cargo",
			"pip":    "pip",
			"poetry": "poetry",
			"maven":  "maven",
			"gradle": "gradle",
			"make":   "make",
			"docker": "docker",
			"git":    "git",
			"kubectl": "kubectl",
		},
	}
}

func (d *Detector) DetectSignals(entries []*parser.ParsedEntry) []Signal {
	var signals []Signal

	for i, entry := range entries {
		if entry.Type == "user" && entry.UserContent != "" {
			var prevAssistant *parser.ParsedEntry
			if i > 0 && entries[i-1].Type == "assistant" {
				prevAssistant = entries[i-1]
			}

			signals = append(signals, d.detectFromUserMessage(entry, prevAssistant)...)
		}

		if entry.Type == "assistant" {
			signals = append(signals, d.detectFromToolUse(entry)...)
		}
	}

	return signals
}

func (d *Detector) detectFromUserMessage(entry *parser.ParsedEntry, prevAssistant *parser.ParsedEntry) []Signal {
	var signals []Signal
	content := strings.TrimSpace(entry.UserContent)

	for _, pattern := range d.approvalPatterns {
		if pattern.Pattern.MatchString(content) {
			sig := Signal{
				Type:      SignalApproval,
				Category:  "approval",
				Key:       "user_approval",
				Value:     content,
				Strength:  pattern.Strength,
				Timestamp: entry.Timestamp,
				Context:   d.getAssistantContext(prevAssistant),
			}
			signals = append(signals, sig)
			break
		}
	}

	for _, pattern := range d.correctionPatterns {
		if matches := pattern.Pattern.FindStringSubmatch(content); matches != nil {
			sig := Signal{
				Type:      SignalCorrection,
				Category:  pattern.Category,
				Key:       d.extractCorrectionKey(matches),
				Value:     content,
				Strength:  pattern.Strength,
				Timestamp: entry.Timestamp,
				Context:   d.getAssistantContext(prevAssistant),
			}
			signals = append(signals, sig)
		}
	}

	signals = append(signals, d.detectExplicitStyleRules(entry)...)

	return signals
}

func (d *Detector) detectExplicitStyleRules(entry *parser.ParsedEntry) []Signal {
	var signals []Signal
	content := strings.ToLower(entry.UserContent)

	stylePatterns := []struct {
		pattern     string
		key         string
		description string
	}{
		{`no.*(end[- ]?of[- ]?line|inline).*(comment|//|#)`, "no_end_of_line_comments", "No end-of-line comments"},
		{`no.*(chinese|mandarin).*(comment|code)`, "no_chinese_comments", "No Chinese in comments"},
		{`use.*(camel\s*case|camelCase)`, "naming_camelCase", "Use camelCase naming"},
		{`use.*(snake[_ ]case|snake_case)`, "naming_snake_case", "Use snake_case naming"},
		{`prefer.*explicit.*error`, "explicit_error_handling", "Prefer explicit error handling"},
		{`no.*emoji`, "no_emoji", "No emojis in code"},
	}

	for _, sp := range stylePatterns {
		if matched, _ := regexp.MatchString(sp.pattern, content); matched {
			signals = append(signals, Signal{
				Type:      SignalStyle,
				Category:  "explicit_style",
				Key:       sp.key,
				Value:     sp.description,
				Strength:  StrengthExplicit,
				Timestamp: entry.Timestamp,
				Context:   entry.UserContent,
			})
		}
	}

	return signals
}

func (d *Detector) detectFromToolUse(entry *parser.ParsedEntry) []Signal {
	var signals []Signal

	for _, tool := range entry.GetToolUses() {
		switch tool.Name {
		case "Write", "Edit":
			signals = append(signals, d.detectStackFromFile(tool, entry)...)
		case "Bash":
			signals = append(signals, d.detectStackFromCommand(tool, entry)...)
		}
	}

	return signals
}

func (d *Detector) detectStackFromFile(tool parser.ContentBlock, entry *parser.ParsedEntry) []Signal {
	var signals []Signal

	var input struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(tool.Input, &input); err != nil || input.FilePath == "" {
		return signals
	}

	ext := filepath.Ext(input.FilePath)
	if lang, ok := d.extensionToLang[ext]; ok {
		signals = append(signals, Signal{
			Type:      SignalStack,
			Category:  "language",
			Key:       lang,
			Value:     input.FilePath,
			Strength:  StrengthWeak,
			Timestamp: entry.Timestamp,
		})
	}

	return signals
}

func (d *Detector) detectStackFromCommand(tool parser.ContentBlock, entry *parser.ParsedEntry) []Signal {
	var signals []Signal

	var input struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(tool.Input, &input); err != nil || input.Command == "" {
		return signals
	}

	parts := strings.Fields(input.Command)
	if len(parts) == 0 {
		return signals
	}

	cmd := parts[0]
	if toolName, ok := d.commandToTool[cmd]; ok {
		signals = append(signals, Signal{
			Type:      SignalStack,
			Category:  "tool",
			Key:       toolName,
			Value:     input.Command,
			Strength:  StrengthWeak,
			Timestamp: entry.Timestamp,
		})
	}

	return signals
}

func (d *Detector) getAssistantContext(entry *parser.ParsedEntry) string {
	if entry == nil {
		return ""
	}

	if text := entry.GetTextContent(); text != "" {
		if len(text) > 200 {
			return text[:200] + "..."
		}
		return text
	}

	tools := entry.GetToolUses()
	if len(tools) > 0 {
		var toolNames []string
		for _, t := range tools {
			toolNames = append(toolNames, t.Name)
		}
		return "Tools used: " + strings.Join(toolNames, ", ")
	}

	return ""
}

func (d *Detector) extractCorrectionKey(matches []string) string {
	if len(matches) > 1 {
		key := strings.TrimSpace(matches[1])
		if len(key) > 50 {
			key = key[:50]
		}
		return key
	}
	return "unknown"
}
