package presentation

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/peauc/lazydocker-ng/pkg/commands"
	"github.com/peauc/lazydocker-ng/pkg/utils"
)

func GetProjectDisplayStrings(project *commands.Project, selectedProjectName string) []string {
	statusIcon := getProjectStatusIcon(project)

	containerInfo := fmt.Sprintf("%d/%d", project.RunningCount, project.ContainerCount)

	path := project.Path
	if len(path) > 30 {
		path = "..." + path[len(path)-27:]
	}

	// Highlight the project name if it's the selected project
	projectName := project.Name
	if selectedProjectName != "" && project.Name == selectedProjectName {
		projectName = utils.ColoredStringDirect(project.Name, color.New(color.FgGreen, color.Bold))
	}

	return []string{
		statusIcon,
		projectName,
		containerInfo,
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
		c = color.FgYellow
	case "mixed":
		icon = "◐"
		c = color.FgYellow
	case "not created":
		icon = "○"
		c = color.FgRed
	default:
		icon = "?"
		c = color.FgWhite
	}

	return utils.ColoredString(icon, c)
}
