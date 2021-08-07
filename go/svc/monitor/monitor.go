package monitor

import (
	"time"
)

type Job struct {
	Name          string        `json:"name"`
	ComponentName string        `json:"component_name"`
	Type          string        `json:"type"`
	Host          string        `json:"host"`
	Interval      time.Duration `json:"interval"`

	ComponentID int
}
