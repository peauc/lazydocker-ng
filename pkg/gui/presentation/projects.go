package presentation

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/peauc/lazydocker-ng/pkg/commands"
	"github.com/peauc/lazydocker-ng/pkg/utils"
)

func GetProjectDisplayStrings(project *commands.Project) []string {
	statusIcon := getProjectStatusIcon(project)

	containerInfo := fmt.Sprintf("%d/%d", project.RunningCount, project.ContainerCount)
	serviceInfo := fmt.Sprintf("%d", project.ServiceCount)

	path := project.Path
	if len(path) > 30 {
		path = "..." + path[len(path)-27:]
	}

	return []string{
		statusIcon,
		project.Name,
		containerInfo,
		serviceInfo,
		utils.ColoredString(path, color.FgCyan),
	}
}

func getProjectStatusIcon(project *commands.Project) string {
	var icon string
	var c color.Attribute

	switch project.Status {
	case "running":
		icon = "●"
		c = color.FgGreen
	case "stopped":
		icon = "○"
		c = color.FgRed
	case "mixed":
		icon = "◐"
		c = color.FgYellow
	default:
		icon = "?"
		c = color.FgWhite
	}

	return utils.ColoredString(icon, c)
}
