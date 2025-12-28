package signals

import (
	"time"
)

type SignalType string

const (
	SignalApproval   SignalType = "approval"
	SignalCorrection SignalType = "correction"
	SignalStack      SignalType = "stack"
	SignalStyle      SignalType = "style"
)

type SignalStrength int

const (
	StrengthWeak     SignalStrength = 1
	StrengthModerate SignalStrength = 2
	StrengthStrong   SignalStrength = 3
	StrengthExplicit SignalStrength = 4
)

type Signal struct {
	Type      SignalType
	Category  string
	Key       string
	Value     string
	Strength  SignalStrength
	Timestamp time.Time
	Context   string
}

type Preference struct {
	Category    string
	Key         string
	Value       string
	Confidence  float64
	SignalCount int
	FirstSeen   time.Time
	LastSeen    time.Time
}

type ConflictingPreference struct {
	Category string
	Key      string
	Values   []ConflictValue
}

type ConflictValue struct {
	Value       string
	Timestamp   time.Time
	SignalCount int
	Strength    float64
}

type FlavorProfile struct {
	Name             string
	CreatedAt        time.Time
	AnalyzedMessages int
	TimeRange        TimeRange

	StackPreferences []Preference
	StylePreferences []Preference
	Approvals        []Preference
	Corrections      []Preference

	Conflicts []ConflictingPreference
}

type TimeRange struct {
	Start time.Time
	End   time.Time
}
