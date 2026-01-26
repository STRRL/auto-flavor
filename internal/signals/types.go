package signals

import (
	"time"
)

type Category string

const (
	CategoryCodeStyle      Category = "code_style"
	CategoryArchitecture   Category = "architecture"
	CategoryTooling        Category = "tooling"
	CategoryVersionControl Category = "version_control"
	CategoryCommunication  Category = "communication"
	CategoryProhibition    Category = "prohibition"
	CategoryStack          Category = "stack"
)

var ValidCategories = map[Category]string{
	CategoryCodeStyle:      "Code style preferences (naming, indentation, comments, formatting)",
	CategoryArchitecture:   "Architecture preferences (patterns, file organization, error handling)",
	CategoryTooling:        "Tooling preferences (package manager, bundler, test framework, linter)",
	CategoryVersionControl: "Version control preferences (commit style, branch naming)",
	CategoryCommunication:  "Communication preferences (response language, verbosity)",
	CategoryProhibition:    "Prohibitions (things explicitly not to do)",
	CategoryStack:          "Technology stack (languages, frameworks, libraries)",
}

func (c Category) IsValid() bool {
	_, ok := ValidCategories[c]
	return ok
}

type Strength string

const (
	StrengthExplicit Strength = "explicit"
	StrengthInferred Strength = "inferred"
)

type Signal struct {
	Category    Category
	Title       string
	Description string
	Strength    Strength
	Timestamp   time.Time
	Context     string
}

type Preference struct {
	Category    Category
	Title       string
	Description string
	Confidence  float64
	SignalCount int
	FirstSeen   time.Time
	LastSeen    time.Time
}

type ConflictingPreference struct {
	Category Category
	Title    string
	Values   []ConflictValue
}

type ConflictValue struct {
	Description string
	Timestamp   time.Time
	SignalCount int
	Strength    float64
}

type FlavorProfile struct {
	CreatedAt        time.Time
	AnalyzedMessages int
	TimeRange        TimeRange
	Preferences      []Preference
	Conflicts        []ConflictingPreference
}

type TimeRange struct {
	Start time.Time
	End   time.Time
}
