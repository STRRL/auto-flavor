package output

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/strrl/auto-flavor/internal/signals"
)

type Generator struct {
	outputDir string
}

func NewGenerator(outputDir string) *Generator {
	return &Generator{
		outputDir: outputDir,
	}
}

func (g *Generator) Generate(profile *signals.FlavorProfile) ([]string, error) {
	flavorDir := filepath.Join(g.outputDir, ".flavor")
	if err := os.MkdirAll(flavorDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .flavor directory: %w", err)
	}

	var files []string

	for _, pref := range profile.StackPreferences {
		filename, err := g.writePreferenceFile(flavorDir, "stack", pref)
		if err != nil {
			return nil, err
		}
		files = append(files, filename)
	}

	for _, pref := range profile.StylePreferences {
		filename, err := g.writePreferenceFile(flavorDir, "style", pref)
		if err != nil {
			return nil, err
		}
		files = append(files, filename)
	}

	for _, pref := range profile.Corrections {
		filename, err := g.writePreferenceFile(flavorDir, "correction", pref)
		if err != nil {
			return nil, err
		}
		files = append(files, filename)
	}

	for _, pref := range profile.Approvals {
		filename, err := g.writePreferenceFile(flavorDir, "approval", pref)
		if err != nil {
			return nil, err
		}
		files = append(files, filename)
	}

	for _, conflict := range profile.Conflicts {
		filename, err := g.writeConflictFile(flavorDir, conflict)
		if err != nil {
			return nil, err
		}
		files = append(files, filename)
	}

	return files, nil
}

func (g *Generator) writePreferenceFile(flavorDir, prefType string, pref signals.Preference) (string, error) {
	safeName := sanitizeFilename(pref.Key)
	filename := filepath.Join(flavorDir, fmt.Sprintf("%s-%s.md", prefType, safeName))

	content := fmt.Sprintf(`# %s: %s

**Category:** %s
**Confidence:** %.1f
**Seen:** %d times
**First seen:** %s
**Last seen:** %s

## Rule

%s
`,
		capitalize(prefType),
		pref.Key,
		pref.Category,
		pref.Confidence,
		pref.SignalCount,
		pref.FirstSeen.Format("2006-01-02"),
		pref.LastSeen.Format("2006-01-02"),
		pref.Value,
	)

	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write %s file: %w", prefType, err)
	}

	return filename, nil
}

func (g *Generator) writeConflictFile(flavorDir string, conflict signals.ConflictingPreference) (string, error) {
	safeName := sanitizeFilename(conflict.Key)
	filename := filepath.Join(flavorDir, fmt.Sprintf("conflict-%s.undecided.md", safeName))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Conflict: %s\n\n", conflict.Key))
	sb.WriteString(fmt.Sprintf("**Category:** %s\n\n", conflict.Category))
	sb.WriteString("This preference has conflicting signals. Please review and decide which to keep.\n\n")

	for i, v := range conflict.Values {
		sb.WriteString(fmt.Sprintf("## Option %d", i+1))
		if i == 0 {
			sb.WriteString(" (Most Recent)")
		}
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("- **Value:** %s\n", truncate(v.Value, 200)))
		sb.WriteString(fmt.Sprintf("- **Last seen:** %s\n", v.Timestamp.Format("2006-01-02")))
		sb.WriteString(fmt.Sprintf("- **Signal count:** %d\n", v.SignalCount))
		sb.WriteString(fmt.Sprintf("- **Strength score:** %.2f\n\n", v.Strength))
	}

	sb.WriteString("---\n\n")
	sb.WriteString("## How to Resolve\n\n")
	sb.WriteString("1. Review each option above\n")
	sb.WriteString("2. Create a new file with your preferred value\n")
	sb.WriteString("3. Delete this file when resolved\n")

	if err := os.WriteFile(filename, []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write conflict file: %w", err)
	}

	return filename, nil
}

func (g *Generator) AppendToClaudeMD(profile *signals.FlavorProfile, targetDir string) error {
	claudeMDPath := filepath.Join(targetDir, "CLAUDE.md")

	var rules []string

	for _, pref := range profile.StylePreferences {
		rules = append(rules, fmt.Sprintf("- %s", pref.Value))
	}

	for _, pref := range profile.Corrections {
		if pref.Category == "prohibition" || pref.Category == "requirement" {
			rules = append(rules, fmt.Sprintf("- %s", pref.Value))
		}
	}

	if len(rules) == 0 {
		return nil
	}

	section := fmt.Sprintf("\n\n## Auto-Flavor Rules\n\n%s\n", strings.Join(rules, "\n"))

	file, err := os.OpenFile(claudeMDPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open CLAUDE.md: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(section); err != nil {
		return fmt.Errorf("failed to append to CLAUDE.md: %w", err)
	}

	return nil
}

func sanitizeFilename(s string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
	result := reg.ReplaceAllString(s, "-")
	result = strings.Trim(result, "-")
	if len(result) > 50 {
		result = result[:50]
	}
	if result == "" {
		result = "unnamed"
	}
	return strings.ToLower(result)
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
