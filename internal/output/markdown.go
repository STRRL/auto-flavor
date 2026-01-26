package output

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/strrl/auto-flavor/internal/signals"
)

type Generator struct {
	outputDir string
}

const categoryMergeThreshold = 3

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

	grouped := groupByCategory(profile.Preferences)
	for category, prefs := range grouped {
		if len(prefs) >= categoryMergeThreshold {
			filename, err := g.writeCategoryFile(flavorDir, category, prefs)
			if err != nil {
				return nil, err
			}
			files = append(files, filename)
			continue
		}

		for _, pref := range prefs {
			filename, err := g.writePreferenceFile(flavorDir, pref)
			if err != nil {
				return nil, err
			}
			files = append(files, filename)
		}
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

func groupByCategory(prefs []signals.Preference) map[signals.Category][]signals.Preference {
	grouped := make(map[signals.Category][]signals.Preference)
	for _, pref := range prefs {
		grouped[pref.Category] = append(grouped[pref.Category], pref)
	}
	return grouped
}

func (g *Generator) writePreferenceFile(flavorDir string, pref signals.Preference) (string, error) {
	safeName := sanitizeFilename(pref.Title)
	filename := filepath.Join(flavorDir, fmt.Sprintf("%s-%s.md", pref.Category, safeName))

	content := fmt.Sprintf(`---
id: %s
category: %s
confidence: %.2f
signal_count: %d
first_seen: %s
last_seen: %s
---

# %s

## Rule

%s
`,
		safeName,
		pref.Category,
		pref.Confidence,
		pref.SignalCount,
		pref.FirstSeen.Format("2006-01-02"),
		pref.LastSeen.Format("2006-01-02"),
		pref.Title,
		pref.Description,
	)

	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write preference file: %w", err)
	}

	return filename, nil
}

func (g *Generator) writeCategoryFile(flavorDir string, category signals.Category, prefs []signals.Preference) (string, error) {
	safeName := sanitizeFilename(string(category))
	filename := filepath.Join(flavorDir, fmt.Sprintf("%s.md", safeName))

	sort.Slice(prefs, func(i, j int) bool {
		return prefs[i].Confidence > prefs[j].Confidence
	})

	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("category: %s\n", category))
	sb.WriteString(fmt.Sprintf("preference_count: %d\n", len(prefs)))
	sb.WriteString("---\n\n")

	sb.WriteString(fmt.Sprintf("# %s\n\n", category))
	sb.WriteString(fmt.Sprintf("%s\n\n", signals.ValidCategories[category]))

	for _, pref := range prefs {
		sb.WriteString(fmt.Sprintf("## %s\n\n", pref.Title))
		sb.WriteString(fmt.Sprintf("- **confidence:** %.2f\n", pref.Confidence))
		sb.WriteString(fmt.Sprintf("- **signal_count:** %d\n", pref.SignalCount))
		sb.WriteString(fmt.Sprintf("- **first_seen:** %s\n", pref.FirstSeen.Format("2006-01-02")))
		sb.WriteString(fmt.Sprintf("- **last_seen:** %s\n\n", pref.LastSeen.Format("2006-01-02")))
		sb.WriteString(fmt.Sprintf("%s\n\n", pref.Description))
	}

	if err := os.WriteFile(filename, []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write category file: %w", err)
	}

	return filename, nil
}

func (g *Generator) writeConflictFile(flavorDir string, conflict signals.ConflictingPreference) (string, error) {
	safeName := sanitizeFilename(conflict.Title)
	filename := filepath.Join(flavorDir, fmt.Sprintf("conflict-%s.undecided.md", safeName))

	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("id: %s\n", safeName))
	sb.WriteString(fmt.Sprintf("category: %s\n", conflict.Category))
	sb.WriteString("status: undecided\n")
	sb.WriteString(fmt.Sprintf("option_count: %d\n", len(conflict.Values)))
	sb.WriteString("---\n\n")

	sb.WriteString(fmt.Sprintf("# Conflict: %s\n\n", conflict.Title))
	sb.WriteString("This preference has conflicting signals. Please review and decide which to keep.\n\n")

	for i, v := range conflict.Values {
		sb.WriteString(fmt.Sprintf("## Option %d", i+1))
		if i == 0 {
			sb.WriteString(" (Most Recent)")
		}
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("- **description:** %s\n", truncate(v.Description, 200)))
		sb.WriteString(fmt.Sprintf("- **last_seen:** %s\n", v.Timestamp.Format("2006-01-02")))
		sb.WriteString(fmt.Sprintf("- **signal_count:** %d\n", v.SignalCount))
		sb.WriteString(fmt.Sprintf("- **strength:** %.2f\n\n", v.Strength))
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

	for _, pref := range profile.Preferences {
		if pref.Category == signals.CategoryProhibition ||
			pref.Category == signals.CategoryCodeStyle ||
			pref.Category == signals.CategoryCommunication {
			rules = append(rules, fmt.Sprintf("- %s", pref.Description))
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
