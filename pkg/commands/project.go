package commands

import "time"

type Project struct {
	Name            string
	Path            string
	ComposeFile     string
	IsDockerCompose bool
	ContainerCount  int
	ServiceCount    int
	RunningCount    int
	Status          string // "running", "stopped", "mixed", "unknown"
	LastUpdated     time.Time
}
