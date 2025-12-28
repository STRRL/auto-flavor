package output

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/strrl/auto-flavor/internal/signals"
)

const flavorTemplate = `# Flavor: {{.Name}}

Generated: {{.CreatedAt.Format "2006-01-02 15:04:05"}}
Analyzed: {{.AnalyzedMessages}} messages
Time range: {{.TimeRange.Start.Format "2006-01-02"}} to {{.TimeRange.End.Format "2006-01-02"}}

## Summary

This flavor profile was extracted from Claude Code chat history.

{{if .StylePreferences}}
## Code Style

{{range .StylePreferences}}
- **{{.Key}}**: {{.Value}} (confidence: {{printf "%.1f" .Confidence}}, seen {{.SignalCount}} times)
{{end}}
{{end}}

{{if .StackPreferences}}
## Preferred Stacks

{{range .StackPreferences}}
- **{{.Key}}**: used {{.SignalCount}} times (confidence: {{printf "%.1f" .Confidence}})
{{end}}
{{end}}

{{if .Corrections}}
## Corrections Made

These are patterns the user corrected:

{{range .Corrections}}
- **{{.Category}}/{{.Key}}**: {{truncate .Value 100}} ({{.SignalCount}} times)
{{end}}
{{end}}

{{if .Approvals}}
## Positive Patterns

Things the user approved:

{{range .Approvals}}
- Approved: {{truncate .Value 100}} ({{.SignalCount}} times)
{{end}}
{{end}}
`

const ambiguousTemplate = `# Ambiguous Preferences: {{.Name}}

These preferences had conflicting signals during analysis. Please review and update
the main flavor file with your decisions.

{{range .Conflicts}}
## {{.Category}}: {{.Key}}

{{range $i, $v := .Values}}
### Option {{add $i 1}}{{if eq $i 0}} (Most Recent){{end}}
- **Value:** {{truncate $v.Value 200}}
- **Last seen:** {{$v.Timestamp.Format "2006-01-02"}}
- **Signal count:** {{$v.SignalCount}}
- **Strength score:** {{printf "%.2f" $v.Strength}}

{{end}}
---

{{end}}

## How to Resolve

1. Review each conflict above
2. Decide which preference you want to keep
3. Add the preferred option to ` + "`{{.Name}}.md`" + `
4. Delete this file when all conflicts are resolved
`

const claudeMDSection = `

## Flavor: %s

%s
`

type Generator struct {
	outputDir string
}

func NewGenerator(outputDir string) *Generator {
	return &Generator{
		outputDir: outputDir,
	}
}

func (g *Generator) Generate(profile *signals.FlavorProfile) error {
	flavorDir := filepath.Join(g.outputDir, ".flavor")
	if err := os.MkdirAll(flavorDir, 0755); err != nil {
		return fmt.Errorf("failed to create .flavor directory: %w", err)
	}

	if err := g.writeFlavorFile(profile, flavorDir); err != nil {
		return err
	}

	if len(profile.Conflicts) > 0 {
		if err := g.writeAmbiguousFile(profile, flavorDir); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) writeFlavorFile(profile *signals.FlavorProfile, flavorDir string) error {
	filename := filepath.Join(flavorDir, profile.Name+".md")

	funcs := template.FuncMap{
		"truncate": truncate,
		"add": func(a, b int) int { return a + b },
	}

	tmpl, err := template.New("flavor").Funcs(funcs).Parse(flavorTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse flavor template: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create flavor file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, profile); err != nil {
		return fmt.Errorf("failed to write flavor file: %w", err)
	}

	return nil
}

func (g *Generator) writeAmbiguousFile(profile *signals.FlavorProfile, flavorDir string) error {
	filename := filepath.Join(flavorDir, profile.Name+".undecided_ambiguous.md")

	funcs := template.FuncMap{
		"truncate": truncate,
		"add": func(a, b int) int { return a + b },
	}

	tmpl, err := template.New("ambiguous").Funcs(funcs).Parse(ambiguousTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse ambiguous template: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create ambiguous file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, profile); err != nil {
		return fmt.Errorf("failed to write ambiguous file: %w", err)
	}

	return nil
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

	section := fmt.Sprintf(claudeMDSection, profile.Name, strings.Join(rules, "\n"))

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

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
