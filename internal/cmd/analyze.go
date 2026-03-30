package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/strrl/auto-flavor/internal/aggregator"
	"github.com/strrl/auto-flavor/internal/output"
	"github.com/strrl/auto-flavor/internal/parser"
	"github.com/strrl/auto-flavor/internal/pipeline"
)

var (
	analyzePath       string
	analyzeDays       int
	analyzeAll        bool
	analyzeApply      bool
	analyzeModel      string
	analyzeMaxEntries int
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze Claude Code chat history to extract coding flavor",
	Long: `Analyze Claude Code chat history for a project to extract the developer's
coding preferences, style patterns, and corrections. Generates flavor files
in .flavor/ directory and optionally appends rules to CLAUDE.md.`,
	RunE: runAnalyze,
}

func init() {
	rootCmd.AddCommand(analyzeCmd)

	analyzeCmd.Flags().StringVarP(&analyzePath, "path", "p", "", "Path to the project to analyze (default: current directory)")
	analyzeCmd.Flags().IntVarP(&analyzeDays, "days", "d", 30, "Number of days to analyze")
	analyzeCmd.Flags().BoolVar(&analyzeAll, "all", false, "Analyze all sessions regardless of time")
	analyzeCmd.Flags().BoolVar(&analyzeApply, "apply", false, "Also append to target project's CLAUDE.md")
	analyzeCmd.Flags().StringVar(&analyzeModel, "model", "google/gemini-2.0-flash-001", "OpenRouter model to use")
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
		return nil
	}

	if analyzeMaxEntries > 0 && len(entries) > analyzeMaxEntries {
		fmt.Printf("Limiting to %d entries for debugging\n", analyzeMaxEntries)
		entries = entries[:analyzeMaxEntries]
	}

	pipe, err := pipeline.New(pipeline.Config{
		Model: analyzeModel,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize pipeline: %w", err)
	}

	fmt.Printf("Using OpenRouter model: %s\n", analyzeModel)
	fmt.Println("Running 3-stage pipeline: Filter → Classify → Extract")

	ctx := context.Background()
	sigs, stats, err := pipe.Process(ctx, entries)
	if err != nil {
		return fmt.Errorf("pipeline failed: %w", err)
	}

	fmt.Printf("Pipeline stats:\n")
	fmt.Printf("  - Total entries: %d\n", stats.TotalEntries)
	fmt.Printf("  - User messages: %d\n", stats.UserEntries)
	fmt.Printf("  - After filter (Stage 1): %d candidates\n", stats.FilteredCount)
	fmt.Printf("  - After classify (Stage 2): %d classified\n", stats.ClassifiedCount)
	fmt.Printf("  - After extract (Stage 3): %d signals\n", stats.ExtractedCount)

	agg := aggregator.NewAggregator(aggregator.DefaultConfig())
	profile := agg.Aggregate(sigs)

	fmt.Printf("Aggregated into profile with:\n")
	fmt.Printf("  - %d preferences\n", len(profile.Preferences))
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
