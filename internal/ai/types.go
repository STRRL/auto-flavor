package ai

import "time"

type Config struct {
	Model       string
	BatchSize   int
	Temperature float64
	Timeout     time.Duration
}

type Entry struct {
	Role      string
	Timestamp time.Time
	Content   string
	Tools     []ToolUse
}

type ToolUse struct {
	Name  string
	Input string
}

type Output struct {
	Signals []OutputSignal `json:"signals"`
}

type OutputSignal struct {
	Type      string `json:"type"`
	Group     string `json:"group"`
	Category  string `json:"category"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	Strength  string `json:"strength"`
	Timestamp string `json:"timestamp"`
	Context   string `json:"context"`
}
