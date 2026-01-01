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
		MinSignalCount:    2,
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

	for key, groupedSigs := range grouped {
		pref, conflict := a.aggregateGroup(key, groupedSigs)

		if conflict != nil {
			profile.Conflicts = append(profile.Conflicts, *conflict)
		} else if pref != nil {
			a.categorizePreference(profile, pref, groupedSigs[0].Type)
		}
	}

	a.sortByConfidence(profile)

	return profile
}

func (a *Aggregator) groupSignals(sigs []signals.Signal) map[string][]signals.Signal {
	groups := make(map[string][]signals.Signal)

	for _, sig := range sigs {
		key := string(sig.Type) + "::" + sig.Category + "::" + sig.Key
		groups[key] = append(groups[key], sig)
	}

	return groups
}

func (a *Aggregator) aggregateGroup(_ string, sigs []signals.Signal) (*signals.Preference, *signals.ConflictingPreference) {
	if len(sigs) < a.config.MinSignalCount {
		if len(sigs) > 0 && sigs[0].Strength == signals.StrengthExplicit {
			return a.buildPreference(sigs), nil
		}
		return nil, nil
	}

	valueGroups := a.groupByValue(sigs)

	if len(valueGroups) > 1 && a.hasConflict(valueGroups) {
		return nil, a.buildConflict(sigs[0].Category, sigs[0].Key, valueGroups)
	}

	return a.buildPreference(sigs), nil
}

func (a *Aggregator) groupByValue(sigs []signals.Signal) map[string][]signals.Signal {
	groups := make(map[string][]signals.Signal)

	for _, sig := range sigs {
		groups[sig.Value] = append(groups[sig.Value], sig)
	}

	return groups
}

func (a *Aggregator) hasConflict(valueGroups map[string][]signals.Signal) bool {
	if len(valueGroups) <= 1 {
		return false
	}

	var scores []float64
	for _, sigs := range valueGroups {
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
		strength := float64(sig.Strength) / 4.0

		score := (a.config.RecencyWeight * recency) +
			(a.config.StrengthWeight * strength)

		totalScore += score
	}

	frequency := float64(len(sigs))
	totalScore += a.config.FrequencyWeight * frequency

	return totalScore
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
		Group:       mostRecent.Group,
		Category:    mostRecent.Category,
		Key:         mostRecent.Key,
		Value:       mostRecent.Value,
		Confidence:  a.calculateGroupScore(sigs),
		SignalCount: len(sigs),
		FirstSeen:   oldest.Timestamp,
		LastSeen:    mostRecent.Timestamp,
	}
}

func (a *Aggregator) buildConflict(category, key string, valueGroups map[string][]signals.Signal) *signals.ConflictingPreference {
	conflict := &signals.ConflictingPreference{
		Group:    pickConflictGroup(valueGroups),
		Category: category,
		Key:      key,
	}

	for value, sigs := range valueGroups {
		if len(sigs) == 0 {
			continue
		}

		sort.Slice(sigs, func(i, j int) bool {
			return sigs[i].Timestamp.After(sigs[j].Timestamp)
		})

		conflict.Values = append(conflict.Values, signals.ConflictValue{
			Value:       value,
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

func pickConflictGroup(valueGroups map[string][]signals.Signal) string {
	for _, sigs := range valueGroups {
		if len(sigs) == 0 {
			continue
		}
		if sigs[0].Group != "" {
			return sigs[0].Group
		}
	}
	return ""
}

func (a *Aggregator) categorizePreference(profile *signals.FlavorProfile, pref *signals.Preference, sigType signals.SignalType) {
	switch sigType {
	case signals.SignalStack:
		profile.StackPreferences = append(profile.StackPreferences, *pref)
	case signals.SignalStyle:
		profile.StylePreferences = append(profile.StylePreferences, *pref)
	case signals.SignalApproval:
		profile.Approvals = append(profile.Approvals, *pref)
	case signals.SignalCorrection:
		profile.Corrections = append(profile.Corrections, *pref)
	}
}

func (a *Aggregator) sortByConfidence(profile *signals.FlavorProfile) {
	sort.Slice(profile.StackPreferences, func(i, j int) bool {
		return profile.StackPreferences[i].Confidence > profile.StackPreferences[j].Confidence
	})

	sort.Slice(profile.StylePreferences, func(i, j int) bool {
		return profile.StylePreferences[i].Confidence > profile.StylePreferences[j].Confidence
	})

	sort.Slice(profile.Corrections, func(i, j int) bool {
		return profile.Corrections[i].Confidence > profile.Corrections[j].Confidence
	})

	sort.Slice(profile.Approvals, func(i, j int) bool {
		return profile.Approvals[i].Confidence > profile.Approvals[j].Confidence
	})
}
