package gui

import (
	"fmt"

	"github.com/jesseduffield/gocui"
	"github.com/peauc/lazydocker-ng/pkg/commands"
	"github.com/peauc/lazydocker-ng/pkg/gui/types"
)

func (gui *Gui) handleDockerContextMenu(g *gocui.Gui, v *gocui.View) error {
	if gui.switchingContext.Load() {
		return nil
	}

	if os.Getenv("DOCKER_HOST") != "" {
		return gui.createErrorPanel("Cannot switch Docker context: DOCKER_HOST environment variable is set and takes precedence over context selection.")
	}

	contexts, err := gui.DockerCommand.ListContexts()
	if err != nil {
		return gui.createErrorPanel(err.Error())
	}

	menuItems := make([]*types.MenuItem, len(contexts))
	for i, ctx := range contexts {
		name := ctx.Name
		if ctx.IsCurrent {
			name = "* " + name
		}
		menuItems[i] = &types.MenuItem{
			LabelColumns: []string{name, ctx.Endpoint},
			OnPress: func() error {
				if ctx.IsCurrent {
					return nil
				}
				return gui.switchDockerContext(ctx.Name)
			},
		}
	}

	return gui.Menu(CreateMenuOptions{
		Title: gui.Tr.DockerContextTitle,
		Items: menuItems,
	})
}

func (gui *Gui) switchDockerContext(contextName string) error {
	if !gui.switchingContext.CompareAndSwap(false, true) {
		return nil
	}

	return gui.WithWaitingStatus(gui.Tr.SwitchingContextStatus, func() error {
		defer gui.switchingContext.Store(false)

		// Stop background goroutines that use the old client.
		// This cancels listenForEvents, monitorContainerStats, and all
		// stat monitor goroutines spawned from them.
		gui.backgroundCancel()

		// Pause goEvery loops to prevent concurrent access during switch
		gui.PauseBackgroundThreads.Store(true)
		defer gui.PauseBackgroundThreads.Store(false)

		// Clear stale data from all panels synchronously.
		// RerenderList (called inside clearAllPanelData) schedules a gui.g.Update
		// internally, but we also need the data cleared before the client swap.
		gui.clearAllPanelData()

		// Switch the Docker client (fail-safe: old client stays if this errors)
		if err := gui.DockerCommand.SwitchContext(contextName); err != nil {
			// Restart goroutines with the old (still valid) client
			gui.restartBackgroundGoroutines()
			// Repopulate panels with the old context's data
			gui.triggerRefresh()
			return fmt.Errorf("failed to switch to context %q: %w", contextName, err)
		}

		// Reset Docker Compose state — the new context may have different
		// (or no) compose projects.
		gui.StateMutex.Lock()
		gui.State.InDockerComposeMode = false
		gui.State.CurrentDockerComposeProject = ""
		gui.State.Project = nil
		gui.StateMutex.Unlock()

		// Restart background goroutines with the new client
		gui.restartBackgroundGoroutines()

		// Update the information bar to show the new context name
		gui.g.Update(func(g *gocui.Gui) error {
			return gui.renderString(gui.g, "information", gui.getInformationContent())
		})

		// Trigger a full refresh of all panels
		gui.triggerRefresh()

		return nil
	})
}
