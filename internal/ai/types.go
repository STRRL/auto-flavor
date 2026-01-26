package ai

import "time"

type Config struct {
	Model       string
	Temperature float64
	Timeout     time.Duration
}
