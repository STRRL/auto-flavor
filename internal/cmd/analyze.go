package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/strrl/auto-flavor/internal/aggregator"
	"github.com/strrl/auto-flavor/internal/ai"
	"github.com/strrl/auto-flavor/internal/output"
	"github.com/strrl/auto-flavor/internal/parser"
)

var (
	analyzePath       string
	analyzeDays       int
	analyzeAll        bool
	analyzeApply      bool
	analyzeModel      string
	analyzeBatchSize  int
	analyzeMaxEntries int
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze Claude Code chat history to extract coding flavor",
	Long: `Analyze Claude Code chat history for a project to extract the developer's
coding preferences, style patterns, and corrections. Generates one flavor file
per rule in .flavor/<rule>.md and optionally appends rules to CLAUDE.md.`,
	RunE: runAnalyze,
}

func init() {
	rootCmd.AddCommand(analyzeCmd)

	analyzeCmd.Flags().StringVarP(&analyzePath, "path", "p", "", "Path to the project to analyze (default: current directory)")
	analyzeCmd.Flags().IntVarP(&analyzeDays, "days", "d", 30, "Number of days to analyze")
	analyzeCmd.Flags().BoolVar(&analyzeAll, "all", false, "Analyze all sessions regardless of time")
	analyzeCmd.Flags().BoolVar(&analyzeApply, "apply", false, "Also append to target project's CLAUDE.md")
	analyzeCmd.Flags().StringVar(&analyzeModel, "model", "google/gemini-3-flash-preview", "OpenRouter model to use")
	analyzeCmd.Flags().IntVar(&analyzeBatchSize, "batch-size", 30, "Number of entries per AI request")
	analyzeCmd.Flags().IntVar(&analyzeMaxEntries, "max-entries", 0, "Max entries to process for debugging (0 = no limit)")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	projectPath, err := resolveProjectPath(analyzePath)
	if err != nil {
		return err
	}

	fmt.Printf("Analyzing project: %s\n", projectPath)
	fmt.Printf("Output directory: %s/.flavor/\n", projectPath)

	p, err := parser.NewParser()
	if err != nil {
		return fmt.Errorf("failed to create parser: %w", err)
	}

	count, first, last, err := p.GetProjectStats(projectPath)
	if err != nil {
		return fmt.Errorf("failed to get project stats: %w", err)
	}

	if count == 0 {
		return fmt.Errorf("no chat history found for project: %s", projectPath)
	}

	fmt.Printf("Found %d messages from %s to %s\n", count, first.Format("2006-01-02"), last.Format("2006-01-02"))

	var since time.Time
	if !analyzeAll {
		since = time.Now().AddDate(0, 0, -analyzeDays)
		fmt.Printf("Analyzing last %d days (since %s)\n", analyzeDays, since.Format("2006-01-02"))
	} else {
		fmt.Println("Analyzing all history")
	}

	entries, err := p.FetchEntriesForProject(projectPath, since)
	if err != nil {
		return fmt.Errorf("failed to fetch entries: %w", err)
	}

	fmt.Printf("Fetched %d entries for analysis\n", len(entries))
	if !analyzeAll {
		fmt.Printf("Entry window since: %s\n", since.Format("2006-01-02"))
	}
	if len(entries) == 0 {
		fmt.Printf("No entries in window. Project history: %s to %s\n", first.Format("2006-01-02"), last.Format("2006-01-02"))
	}

	if analyzeMaxEntries > 0 && len(entries) > analyzeMaxEntries {
		fmt.Printf("Limiting to %d entries for debugging\n", analyzeMaxEntries)
		entries = entries[:analyzeMaxEntries]
	}

	detector, err := ai.NewDetector(ai.Config{
		Model:     analyzeModel,
		BatchSize: analyzeBatchSize,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize AI detector: %w", err)
	}

	fmt.Printf("Using OpenRouter model: %s\n", analyzeModel)
	fmt.Printf("AI batch size: %d\n", analyzeBatchSize)

	sigs, err := detector.DetectSignals(entries)
	if err != nil {
		return fmt.Errorf("failed to detect signals: %w", err)
	}

	fmt.Printf("Detected %d signals\n", len(sigs))

	agg := aggregator.NewAggregator(aggregator.DefaultConfig())
	profile := agg.Aggregate(sigs)

	fmt.Printf("Aggregated into profile with:\n")
	fmt.Printf("  - %d stack preferences\n", len(profile.StackPreferences))
	fmt.Printf("  - %d style preferences\n", len(profile.StylePreferences))
	fmt.Printf("  - %d corrections\n", len(profile.Corrections))
	fmt.Printf("  - %d approvals\n", len(profile.Approvals))
	fmt.Printf("  - %d conflicts\n", len(profile.Conflicts))

	gen := output.NewGenerator(projectPath)
	files, err := gen.Generate(profile)
	if err != nil {
		return fmt.Errorf("failed to generate output: %w", err)
	}

	fmt.Printf("Generated %d flavor files in .flavor/\n", len(files))
	for _, f := range files {
		fmt.Printf("  - %s\n", f)
	}

	if analyzeApply {
		if err := gen.AppendToClaudeMD(profile, projectPath); err != nil {
			return fmt.Errorf("failed to append to CLAUDE.md: %w", err)
		}
		fmt.Printf("Appended flavor rules to %s/CLAUDE.md\n", projectPath)
	}

	return nil
}

func resolveProjectPath(path string) (string, error) {
	if path == "" {
		return os.Getwd()
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("path does not exist: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", absPath)
	}

	return absPath, nil
}
