package aggregator

import (
	"sort"
	"time"

	"github.com/strrl/auto-flavor/internal/signals"
)

type Config struct {
	RecencyWeight     float64
	FrequencyWeight   float64
	StrengthWeight    float64
	MinSignalCount    int
	ConflictThreshold float64
	RecencyDecayDays  int
}

func DefaultConfig() Config {
	return Config{
		RecencyWeight:     0.4,
		FrequencyWeight:   0.3,
		StrengthWeight:    0.3,
		MinSignalCount:    1,
		ConflictThreshold: 0.3,
		RecencyDecayDays:  30,
	}
}

type Aggregator struct {
	config Config
	now    time.Time
}

func NewAggregator(cfg Config) *Aggregator {
	return &Aggregator{
		config: cfg,
		now:    time.Now(),
	}
}

func (a *Aggregator) Aggregate(sigs []signals.Signal) *signals.FlavorProfile {
	profile := &signals.FlavorProfile{
		CreatedAt:        a.now,
		AnalyzedMessages: len(sigs),
	}

	if len(sigs) > 0 {
		profile.TimeRange = signals.TimeRange{
			Start: sigs[0].Timestamp,
			End:   sigs[len(sigs)-1].Timestamp,
		}
	}

	grouped := a.groupSignals(sigs)

	for _, groupedSigs := range grouped {
		pref, conflict := a.aggregateGroup(groupedSigs)

		if conflict != nil {
			profile.Conflicts = append(profile.Conflicts, *conflict)
		} else if pref != nil {
			profile.Preferences = append(profile.Preferences, *pref)
		}
	}

	a.sortByConfidence(profile)

	return profile
}

func (a *Aggregator) groupSignals(sigs []signals.Signal) map[string][]signals.Signal {
	groups := make(map[string][]signals.Signal)

	for _, sig := range sigs {
		key := string(sig.Category) + "::" + sig.Title
		groups[key] = append(groups[key], sig)
	}

	return groups
}

func (a *Aggregator) aggregateGroup(sigs []signals.Signal) (*signals.Preference, *signals.ConflictingPreference) {
	if len(sigs) < a.config.MinSignalCount {
		return nil, nil
	}

	descGroups := a.groupByDescription(sigs)

	if len(descGroups) > 1 && a.hasConflict(descGroups) {
		return nil, a.buildConflict(sigs[0].Category, sigs[0].Title, descGroups)
	}

	return a.buildPreference(sigs), nil
}

func (a *Aggregator) groupByDescription(sigs []signals.Signal) map[string][]signals.Signal {
	groups := make(map[string][]signals.Signal)

	for _, sig := range sigs {
		groups[sig.Description] = append(groups[sig.Description], sig)
	}

	return groups
}

func (a *Aggregator) hasConflict(descGroups map[string][]signals.Signal) bool {
	if len(descGroups) <= 1 {
		return false
	}

	var scores []float64
	for _, sigs := range descGroups {
		score := a.calculateGroupScore(sigs)
		scores = append(scores, score)
	}

	sort.Float64s(scores)
	if len(scores) < 2 {
		return false
	}

	topScore := scores[len(scores)-1]
	secondScore := scores[len(scores)-2]

	if topScore == 0 {
		return false
	}

	return secondScore/topScore >= a.config.ConflictThreshold
}

func (a *Aggregator) calculateGroupScore(sigs []signals.Signal) float64 {
	var totalScore float64

	for _, sig := range sigs {
		recency := a.recencyScore(sig.Timestamp)
		strength := strengthToFloat(sig.Strength)

		score := (a.config.RecencyWeight * recency) +
			(a.config.StrengthWeight * strength)

		totalScore += score
	}

	frequency := float64(len(sigs))
	totalScore += a.config.FrequencyWeight * frequency

	return totalScore
}

func strengthToFloat(s signals.Strength) float64 {
	if s == signals.StrengthExplicit {
		return 1.0
	}
	return 0.5
}

func (a *Aggregator) recencyScore(timestamp time.Time) float64 {
	daysSince := a.now.Sub(timestamp).Hours() / 24

	if daysSince >= float64(a.config.RecencyDecayDays) {
		return 0.1
	}

	return 1.0 - (daysSince / float64(a.config.RecencyDecayDays) * 0.9)
}

func (a *Aggregator) buildPreference(sigs []signals.Signal) *signals.Preference {
	sort.Slice(sigs, func(i, j int) bool {
		return sigs[i].Timestamp.After(sigs[j].Timestamp)
	})

	mostRecent := sigs[0]
	oldest := sigs[len(sigs)-1]

	return &signals.Preference{
		Category:    mostRecent.Category,
		Title:       mostRecent.Title,
		Description: mostRecent.Description,
		Confidence:  a.calculateGroupScore(sigs),
		SignalCount: len(sigs),
		FirstSeen:   oldest.Timestamp,
		LastSeen:    mostRecent.Timestamp,
	}
}

func (a *Aggregator) buildConflict(category signals.Category, title string, descGroups map[string][]signals.Signal) *signals.ConflictingPreference {
	conflict := &signals.ConflictingPreference{
		Category: category,
		Title:    title,
	}

	for desc, sigs := range descGroups {
		if len(sigs) == 0 {
			continue
		}

		sort.Slice(sigs, func(i, j int) bool {
			return sigs[i].Timestamp.After(sigs[j].Timestamp)
		})

		conflict.Values = append(conflict.Values, signals.ConflictValue{
			Description: desc,
			Timestamp:   sigs[0].Timestamp,
			SignalCount: len(sigs),
			Strength:    a.calculateGroupScore(sigs),
		})
	}

	sort.Slice(conflict.Values, func(i, j int) bool {
		return conflict.Values[i].Timestamp.After(conflict.Values[j].Timestamp)
	})

	return conflict
}

func (a *Aggregator) sortByConfidence(profile *signals.FlavorProfile) {
	sort.Slice(profile.Preferences, func(i, j int) bool {
		return profile.Preferences[i].Confidence > profile.Preferences[j].Confidence
	})
}
