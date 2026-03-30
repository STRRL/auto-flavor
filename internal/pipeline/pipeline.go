package pipeline

import (
	"context"
	"fmt"

	"github.com/strrl/auto-flavor/internal/ai"
	"github.com/strrl/auto-flavor/internal/parser"
	"github.com/strrl/auto-flavor/internal/signals"
)

type Pipeline struct {
	filter     *Filter
	classifier *Classifier
	extractor  *Extractor
}

type Config struct {
	Model       string
	Temperature float64
}

func New(cfg Config) (*Pipeline, error) {
	client, err := ai.NewClient(ai.Config{
		Model:       cfg.Model,
		Temperature: cfg.Temperature,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AI client: %w", err)
	}

	return &Pipeline{
		filter:     NewFilter(client),
		classifier: NewClassifier(client),
		extractor:  NewExtractor(client),
	}, nil
}

type Stats struct {
	TotalEntries    int
	UserEntries     int
	FilteredCount   int
	ClassifiedCount int
	ExtractedCount  int
}

func (p *Pipeline) Process(ctx context.Context, entries []*parser.ParsedEntry) ([]signals.Signal, Stats, error) {
	stats := Stats{
		TotalEntries: len(entries),
	}

	userEntries := filterUserEntries(entries)
	stats.UserEntries = len(userEntries)

	if len(userEntries) == 0 {
		return nil, stats, nil
	}

	candidates, err := p.filter.Filter(ctx, userEntries)
	if err != nil {
		return nil, stats, fmt.Errorf("filter failed: %w", err)
	}
	stats.FilteredCount = len(candidates)

	if len(candidates) == 0 {
		return nil, stats, nil
	}

	classified, err := p.classifier.Classify(ctx, candidates)
	if err != nil {
		return nil, stats, fmt.Errorf("classification failed: %w", err)
	}
	stats.ClassifiedCount = len(classified)

	if len(classified) == 0 {
		return nil, stats, nil
	}

	sigs, err := p.extractor.Extract(ctx, classified)
	if err != nil {
		return nil, stats, fmt.Errorf("extraction failed: %w", err)
	}
	stats.ExtractedCount = len(sigs)

	return sigs, stats, nil
}

func filterUserEntries(entries []*parser.ParsedEntry) []*parser.ParsedEntry {
	var result []*parser.ParsedEntry
	for _, e := range entries {
		if e.Type == "user" {
			result = append(result, e)
		}
	}
	return result
}
