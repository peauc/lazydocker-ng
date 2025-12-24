package gui

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/peauc/lazydocker-ng/pkg/gui/types"

	"github.com/fatih/color"
	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/yaml"
	"github.com/peauc/lazydocker-ng/pkg/commands"
	"github.com/peauc/lazydocker-ng/pkg/gui/panels"
	"github.com/peauc/lazydocker-ng/pkg/gui/presentation"
	"github.com/peauc/lazydocker-ng/pkg/tasks"
	"github.com/peauc/lazydocker-ng/pkg/utils"
)

// Although at the moment we'll only have one project, in future we could have
// a list of projects in the project panel.

func (gui *Gui) getProjectPanel() *panels.SideListPanel[*commands.Project] {
	return &panels.SideListPanel[*commands.Project]{
		ContextState: &panels.ContextState[*commands.Project]{
			GetMainTabs: func() []panels.MainTab[*commands.Project] {
				if gui.DockerCommand.InDockerComposeProject {
					return []panels.MainTab[*commands.Project]{
						{
							Key:    "logs",
							Title:  gui.Tr.LogsTitle,
							Render: gui.renderAllLogs,
						},
						{
							Key:    "config",
							Title:  gui.Tr.DockerComposeConfigTitle,
							Render: gui.renderDockerComposeConfig,
						},
						{
							Key:    "credits",
							Title:  gui.Tr.CreditsTitle,
							Render: gui.renderCredits,
						},
					}
				}

				return []panels.MainTab[*commands.Project]{
					{
						Key:    "credits",
						Title:  gui.Tr.CreditsTitle,
						Render: gui.renderCredits,
					},
				}
			},
			GetItemContextCacheKey: func(project *commands.Project) string {
				return "projects-" + project.Name
			},
		},

		ListPanel: panels.ListPanel[*commands.Project]{
			List: panels.NewFilteredList[*commands.Project](),
			View: gui.Views.Project,
		},
		NoItemsMessage: "No docker compose projects found.",
		Gui:            gui.intoInterface(),

		Sort: func(a *commands.Project, b *commands.Project) bool {
			return (gui.State.Project != nil && gui.State.Project.Name == a.Name) || a.Name < b.Name
		},
		GetTableCells: func(project *commands.Project) []string {
			selectedProjectName := ""
			if gui.State.Project != nil {
				selectedProjectName = gui.State.Project.Name
			}
			return presentation.GetProjectDisplayStrings(project, selectedProjectName)
		},
		OnClick: func(project *commands.Project) error {
			return gui.handleProjectSelect(nil, nil)
		},
		Hide: func() bool {
			return gui.State.UIMode != MODE_CONTAINER
		},
	}
}

func (gui *Gui) refreshProjects() error {
	containers, err := gui.DockerCommand.Client.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		return err
	}

	projectsMap := make(map[string]*commands.Project)
	servicesPerProject := make(map[string]map[string]bool)

	// Build project info from containers
	for _, container := range containers {
		projectName, exists := container.Labels["com.docker.compose.project"]
		if !exists || projectName == "" {
			continue
		}

		if _, ok := projectsMap[projectName]; !ok {
			projectsMap[projectName] = &commands.Project{
				Name:            projectName,
				IsDockerCompose: true,
				Status:          "unknown",
				LastUpdated:     time.Now(),
			}
			servicesPerProject[projectName] = make(map[string]bool)
		}

		project := projectsMap[projectName]
		project.ContainerCount++

		if container.State == "running" {
			project.RunningCount++
		}

		if project.Path == "" {
			if workingDir, ok := container.Labels["com.docker.compose.project.working_dir"]; ok {
				project.Path = workingDir
			}
		}

		if serviceName, ok := container.Labels["com.docker.compose.service"]; ok && serviceName != "" {
			servicesPerProject[projectName][serviceName] = true
		}
	}

	projectsList := make([]*commands.Project, 0, len(projectsMap))
	for _, project := range projectsMap {
		projectsList = append(projectsList, project)
	}

	if gui.DockerCommand.StartedInDockerComposeProject {
		currentDirProjectName := path.Base(gui.Config.ProjectDir)
		if _, exists := projectsMap[currentDirProjectName]; !exists {
			projectsList = append(projectsList, &commands.Project{
				Name:            currentDirProjectName,
				Path:            gui.Config.ProjectDir,
				IsDockerCompose: true,
				Status:          "not created",
			})
		}
	}

	for _, project := range projectsMap {
		if project.RunningCount == 0 {
			project.Status = "stopped"
		} else if project.RunningCount == project.ContainerCount {
			project.Status = "running"
		} else {
			project.Status = "mixed"
		}

		project.ServiceCount = len(servicesPerProject[project.Name])
	}

	gui.Panels.Projects.SetItems(projectsList)
	return gui.Panels.Projects.RerenderList()
}

func (gui *Gui) handleProjectSelect(g *gocui.Gui, v *gocui.View) error {
	project, err := gui.Panels.Projects.GetSelectedItem()
	if err != nil {
		gui.Log.Error(err)
		return nil
	}

	gui.Log.Info("Selected project: " + project.Name)

	gui.State.Project = project

	gui.DockerCommand.CurrentDockerComposeProject = project.Name
	gui.DockerCommand.InDockerComposeProject = project.IsDockerCompose

	if err := gui.refreshContainersAndServices(); err != nil {
		gui.Log.Error(err)
		return err
	}

	if err := gui.Panels.Projects.RerenderList(); err != nil {
		return err
	}
	if err := gui.Panels.Services.RerenderList(); err != nil {
		return err
	}
	if err := gui.Panels.Containers.RerenderList(); err != nil {
		return err
	}

	if err := gui.Panels.Projects.HandleSelect(); err != nil {
		return err
	}

	return nil
}

func (gui *Gui) GetProjectName() string {
	if gui.State.Project != nil {
		return gui.State.Project.Name
	}

	// Default to the command line argument
	return path.Base(gui.Config.ProjectDir)
}

func (gui *Gui) renderCredits(_project *commands.Project) tasks.TaskFunc {
	return gui.NewSimpleRenderStringTask(func() string { return gui.creditsStr() })
}

func (gui *Gui) creditsStr() string {
	var configBuf bytes.Buffer
	_ = yaml.NewEncoder(&configBuf, yaml.IncludeOmitted).Encode(gui.Config.UserConfig)

	return strings.Join(
		[]string{
			lazydockerTitle(),
			"Copyright (c) 2019 Jesse Duffield",
			"Keybindings: https://github.com/peauc/lazydocker-ng/blob/master/docs/keybindings",
			"Config Options: https://github.com/peauc/lazydocker-ng/blob/master/docs/Config.md",
			"Raise an Issue: https://github.com/peauc/lazydocker-ng/issues",
			utils.ColoredString("Buy Jesse a coffee: https://github.com/sponsors/jesseduffield", color.FgMagenta), // caffeine ain't free
			"Here's your lazydocker config when merged in with the defaults (you can open your config by pressing 'o'):",
			utils.ColoredYamlString(configBuf.String()),
		}, "\n\n")
}

func (gui *Gui) renderAllLogs(_project *commands.Project) tasks.TaskFunc {
	return gui.NewTask(TaskOpts{
		Autoscroll: true,
		Wrap:       gui.Config.UserConfig.Gui.WrapMainPanel,
		Func: func(ctx context.Context) {
			gui.clearMainView()

			cmd := gui.OSCommand.RunCustomCommand(
				utils.ApplyTemplate(
					gui.Config.UserConfig.CommandTemplates.AllLogs,
					gui.DockerCommand.NewCommandObject(commands.CommandObject{}),
				),
			)

			cmd.Stdout = gui.Views.Main
			cmd.Stderr = gui.Views.Main

			gui.OSCommand.PrepareForChildren(cmd)
			_ = cmd.Start()

			go func() {
				<-ctx.Done()
				if err := gui.OSCommand.Kill(cmd); err != nil {
					gui.Log.Error(err)
				}
			}()

			_ = cmd.Wait()
		},
	})
}

func (gui *Gui) renderDockerComposeConfig(project *commands.Project) tasks.TaskFunc {
	return gui.NewSimpleRenderStringTask(func() string {
		output, err := gui.DockerCommand.DockerComposeConfigForProjectWithError(project.Path)

		if err != nil {
			if commands.IsDockerComposeFileNotFoundError(err) {
				return fmt.Sprintf("%s\n\n%s\n%s",
					utils.ColoredString("No docker-compose file found", color.FgYellow),
					utils.ColoredString("Project: ", color.FgWhite)+project.Name,
					utils.ColoredString("Path: ", color.FgWhite)+project.Path)
			}

			if commands.IsDockerComposeYAMLError(err) {
				return fmt.Sprintf("%s\n\n%s",
					utils.ColoredString("YAML parsing error in compose file", color.FgRed),
					err.Error())
			}

			if commands.IsDockerComposeValidationError(err) {
				return fmt.Sprintf("%s\n\n%s",
					utils.ColoredString("Compose file validation failed", color.FgRed),
					err.Error())
			}

			return fmt.Sprintf("%s\n\n%s",
				utils.ColoredString("Failed to read docker-compose config", color.FgRed),
				err.Error())
		}

		return utils.ColoredYamlString(output)
	})
}

func (gui *Gui) handleOpenConfig(g *gocui.Gui, v *gocui.View) error {
	return gui.openFile(gui.Config.ConfigFilename())
}

func (gui *Gui) handleEditConfig(g *gocui.Gui, v *gocui.View) error {
	return gui.editFile(gui.Config.ConfigFilename())
}

func lazydockerTitle() string {
	return `
   _                     _            _
  | |                   | |          | |
  | | __ _ _____   _  __| | ___   ___| | _____ _ __
  | |/ _` + "`" + ` |_  / | | |/ _` + "`" + ` |/ _ \ / __| |/ / _ \ '__|
  | | (_| |/ /| |_| | (_| | (_) | (__|   <  __/ |
  |_|\__,_/___|\__, |\__,_|\___/ \___|_|\_\___|_|
                __/ |
               |___/
`
}

// handleViewAllLogs switches to a subprocess viewing all the logs from docker-compose
func (gui *Gui) handleViewAllLogs(g *gocui.Gui, v *gocui.View) error {
	c, err := gui.DockerCommand.ViewAllLogs()
	if err != nil {
		return gui.createErrorPanel(err.Error())
	}

	return gui.runSubprocess(c)
}

func (gui *Gui) handleCreateProjectMenu(g *gocui.Gui, v *gocui.View) error {
	if gui.isPopupPanel(v.Name()) {
		return nil
	}

	testMenuItem := &types.MenuItem{
		LabelColumns: []string{"t", "test"},
		OnPress: func() error {
			gui.Log.Println("tested")
			return nil
		},
	}

	menuItems := []*types.MenuItem{testMenuItem}

	return gui.Menu(CreateMenuOptions{
		Title:      gui.Tr.MenuTitle,
		Items:      menuItems,
		HideCancel: true,
	})
}
