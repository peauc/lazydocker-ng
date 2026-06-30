package gui

import (
	"context"
	"fmt"
	"path"

	"github.com/peauc/lazydocker-ng/pkg/gui/types"

	"github.com/fatih/color"
	"github.com/jesseduffield/gocui"
	"github.com/peauc/lazydocker-ng/pkg/commands"
	"github.com/peauc/lazydocker-ng/pkg/gui/panels"
	"github.com/peauc/lazydocker-ng/pkg/gui/presentation"
	"github.com/peauc/lazydocker-ng/pkg/tasks"
	"github.com/peauc/lazydocker-ng/pkg/utils"
	"github.com/samber/lo"
)

// Although at the moment we'll only have one project, in future we could have
// a list of projects in the project panel.

func (gui *Gui) getProjectPanel() *panels.SideListPanel[*commands.Project] {
	return &panels.SideListPanel[*commands.Project]{
		ContextState: &panels.ContextState[*commands.Project]{
			GetMainTabs: func() []panels.MainTab[*commands.Project] {
				if gui.State.InDockerComposeMode {
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
					}
				}

				return []panels.MainTab[*commands.Project]{}
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
			return a.Name < b.Name
		},
		GetTableCells: func(project *commands.Project) []string {
			selectedProjectName := ""
			if gui.State.Project != nil {
				selectedProjectName = gui.State.Project.Name
			}
			return presentation.GetProjectDisplayStrings(project, selectedProjectName)
		},
		OnSelect: gui.selectProject,
		Hide: func() bool {
			return gui.State.UIMode != MODE_CONTAINERS || !gui.State.InDockerComposeMode
		},
	}
}

// selectProject makes the given project the active one and refreshes the views
// that depend on it.
func (gui *Gui) selectProject(project *commands.Project) error {
	gui.Log.Info("Selected project: " + project.Name)

	gui.StateMutex.Lock()
	gui.State.Project = project
	gui.State.CurrentDockerComposeProject = project.Name
	gui.StateMutex.Unlock()

	if err := gui.refreshContainersAndServices(); err != nil {
		gui.Log.Error(err)
		return err
	}

	return gui.Panels.Projects.RerenderList()
}

func (gui *Gui) refreshProjects() error {
	// Get containers using the commands layer
	currentContainers := gui.Panels.Containers.List.GetItems()
	containers, err := gui.DockerCommand.GetContainers(currentContainers)
	if err != nil {
		return err
	}

	// Use the commands layer to extract projects from containers
	projectsList := gui.DockerCommand.GetProjects(
		containers,
		gui.Config.ProjectDir,
		gui.State.InDockerComposeMode,
	)

	gui.Panels.Projects.SetItems(projectsList)

	// Auto-select a project if none is active yet, so the containers and
	// services panels are populated without requiring the user to move the
	// cursor first (selection now happens on cursor move, not via a keypress).
	gui.StateMutex.RLock()
	noProjectSelected := gui.State.Project == nil
	gui.StateMutex.RUnlock()

	if noProjectSelected {
		if project, err := gui.Panels.Projects.GetSelectedItem(); err == nil {
			return gui.selectProject(project)
		}
	}

	return gui.Panels.Projects.RerenderList()
}

func (gui *Gui) GetProjectName() string {
	if gui.State.Project != nil {
		return gui.State.Project.Name
	}

	// Default to the command line argument
	return path.Base(gui.Config.ProjectDir)
}

func (gui *Gui) renderAllLogs(project *commands.Project) tasks.TaskFunc {
	return gui.NewTask(TaskOpts{
		Autoscroll: true,
		Wrap:       gui.Config.UserConfig.Gui.WrapMainPanel,
		Func: func(ctx context.Context) {
			gui.clearMainView()

			cmd := gui.OSCommand.RunCustomCommand(
				utils.ApplyTemplate(
					gui.Config.UserConfig.CommandTemplates.AllLogs,
					gui.DockerCommand.NewCommandObjectWithProjectName(project),
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

// handleViewAllLogs switches to a subprocess viewing all the logs from docker-compose
func (gui *Gui) handleViewAllLogs(g *gocui.Gui, v *gocui.View) error {
	project, err := gui.Panels.Projects.GetSelectedItem()
	if err != nil {
		return nil
	}

	cmdStr := utils.ApplyTemplate(
		gui.Config.UserConfig.CommandTemplates.ViewAllLogs,
		gui.DockerCommand.NewCommandObjectWithProjectName(project),
	)

	c := gui.OSCommand.ExecutableFromString(cmdStr)

	gui.OSCommand.PrepareForChildren(c)

	return gui.runSubprocess(c)
}

type commandOption struct {
	description string
	command     string
	onPress     func() error
}

func (r *commandOption) getDisplayStrings() []string {
	return []string{r.description, color.New(color.FgCyan).Sprint(r.command)}
}

func (gui *Gui) handleProjectUp(g *gocui.Gui, v *gocui.View) error {
	project, err := gui.Panels.Projects.GetSelectedItem()
	if err != nil {
		return nil
	}

	return gui.createConfirmationPanel(gui.Tr.Confirm, gui.Tr.ConfirmUpProject, func(g *gocui.Gui, v *gocui.View) error {
		cmdStr := utils.ApplyTemplate(
			gui.Config.UserConfig.CommandTemplates.Up,
			gui.DockerCommand.NewCommandObjectWithComposeFile(project),
		)

		return gui.WithWaitingStatus(gui.Tr.UppingProjectStatus, func() error {
			cmd := gui.OSCommand.ExecutableFromString(cmdStr)
			cmd.Dir = project.Path
			if err := gui.OSCommand.RunExecutable(cmd); err != nil {
				return gui.createErrorPanel(err.Error())
			}
			return nil
		})
	}, nil)
}

func (gui *Gui) handleProjectStart(g *gocui.Gui, v *gocui.View) error {
	project, err := gui.Panels.Projects.GetSelectedItem()
	if err != nil {
		return nil
	}

	cmdStr := utils.ApplyTemplate(
		gui.Config.UserConfig.CommandTemplates.Start,
		gui.DockerCommand.NewCommandObjectWithComposeFile(project),
	)

	return gui.WithWaitingStatus(gui.Tr.StartingProjectStatus, func() error {
		cmd := gui.OSCommand.ExecutableFromString(cmdStr)
		cmd.Dir = project.Path
		if err := gui.OSCommand.RunExecutable(cmd); err != nil {
			return gui.createErrorPanel(err.Error())
		}
		return nil
	})
}

func (gui *Gui) handleProjectStop(g *gocui.Gui, v *gocui.View) error {
	project, err := gui.Panels.Projects.GetSelectedItem()
	if err != nil {
		return nil
	}

	return gui.createConfirmationPanel(gui.Tr.Confirm, gui.Tr.ConfirmStopProject, func(g *gocui.Gui, v *gocui.View) error {
		cmdStr := utils.ApplyTemplate(
			gui.Config.UserConfig.CommandTemplates.Stop,
			gui.DockerCommand.NewCommandObjectWithComposeFile(project),
		)

		return gui.WithWaitingStatus(gui.Tr.StoppingProjectStatus, func() error {
			cmd := gui.OSCommand.ExecutableFromString(cmdStr)
			cmd.Dir = project.Path
			if err := gui.OSCommand.RunExecutable(cmd); err != nil {
				return gui.createErrorPanel(err.Error())
			}
			return nil
		})
	}, nil)
}

// Get service count from docker-compose.yml

func (gui *Gui) handleProjectDown(g *gocui.Gui, v *gocui.View) error {
	project, err := gui.Panels.Projects.GetSelectedItem()
	if err != nil {
		return nil
	}

	downCommand := utils.ApplyTemplate(
		gui.Config.UserConfig.CommandTemplates.Down,
		gui.DockerCommand.NewCommandObjectWithComposeFile(project),
	)

	downWithVolumesCommand := utils.ApplyTemplate(
		gui.Config.UserConfig.CommandTemplates.DownWithVolumes,
		gui.DockerCommand.NewCommandObjectWithComposeFile(project),
	)

	options := []*commandOption{
		{
			description: gui.Tr.Down,
			command:     downCommand,
			onPress: func() error {
				return gui.WithWaitingStatus(gui.Tr.DowningStatus, func() error {
					cmd := gui.OSCommand.ExecutableFromString(downCommand)
					cmd.Dir = project.Path
					if err := gui.OSCommand.RunExecutable(cmd); err != nil {
						return gui.createErrorPanel(err.Error())
					}
					return nil
				})
			},
		},
		{
			description: gui.Tr.DownWithVolumes,
			command:     downWithVolumesCommand,
			onPress: func() error {
				return gui.WithWaitingStatus(gui.Tr.DowningStatus, func() error {
					cmd := gui.OSCommand.ExecutableFromString(downWithVolumesCommand)
					cmd.Dir = project.Path
					if err := gui.OSCommand.RunExecutable(cmd); err != nil {
						return gui.createErrorPanel(err.Error())
					}
					return nil
				})
			},
		},
	}

	menuItems := lo.Map(options, func(option *commandOption, _ int) *types.MenuItem {
		return &types.MenuItem{
			LabelColumns: option.getDisplayStrings(),
			OnPress:      option.onPress,
		}
	})

	return gui.Menu(CreateMenuOptions{
		Title: "",
		Items: menuItems,
	})
}
