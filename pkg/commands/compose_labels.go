package commands

import "regexp"

var (
	// composeProjectName is the rule docker compose enforces for project names.
	composeProjectName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	// composeServiceName is compose's service-name charset, minus a leading
	// "-" so a name can never be read as a flag.
	composeServiceName = regexp.MustCompile(`^[a-zA-Z0-9._][a-zA-Z0-9._-]*$`)
)

// composeLabels returns a container's compose project and service names, or
// "" for a value docker compose itself would never create. Labels can come
// from any image, and these names end up in compose command lines (including
// user command templates), so anything else is treated as "not compose".
func composeLabels(labels map[string]string) (project, service string) {
	if name := labels["com.docker.compose.project"]; composeProjectName.MatchString(name) {
		project = name
	}
	if name := labels["com.docker.compose.service"]; composeServiceName.MatchString(name) {
		service = name
	}
	return project, service
}
