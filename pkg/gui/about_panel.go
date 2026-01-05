package gui

import (
	"bytes"
	"math"
	"strings"

	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/yaml"
	"github.com/peauc/lazydocker-ng/pkg/utils"
)

// handleOpenAboutPopup opens the About view and hides all other panels
func (gui *Gui) handleOpenAboutPopup(g *gocui.Gui, v *gocui.View) error {
	// Hide all regular panels
	gui.hideAllPanels()

	// Show and populate the About view
	aboutView := gui.Views.About
	aboutView.Visible = true
	aboutView.Title = gui.Tr.AboutTitle
	aboutView.Wrap = true
	aboutView.Autoscroll = false

	// Render the about content
	content := gui.getAboutContent()
	gui.Views.About.Clear()
	if _, err := gui.Views.About.Write([]byte(content)); err != nil {
		return err
	}

	// Reset scroll position
	gui.Views.About.SetOrigin(0, 0)
	gui.Views.About.SetCursor(0, 0)

	// Set focus to About view and trigger UI update
	gui.g.Update(func(g *gocui.Gui) error {
		_, err := gui.g.SetCurrentView("about")
		return err
	})

	return nil
}

// handleCloseAboutPopup closes the About view and restores all panels
func (gui *Gui) handleCloseAboutPopup() error {
	// Hide About view
	gui.Views.About.Visible = false

	// Show all regular panels
	gui.showAllPanels()

	// Return to previous focus and trigger UI update
	gui.g.Update(func(g *gocui.Gui) error {
		return gui.returnFocus()
	})

	return nil
}

// hideAllPanels hides all regular UI panels
func (gui *Gui) hideAllPanels() {
	gui.Views.ModeTabs.Visible = false
	gui.Views.Project.Visible = false
	gui.Views.Services.Visible = false
	gui.Views.Containers.Visible = false
	gui.Views.Images.Visible = false
	gui.Views.Volumes.Visible = false
	gui.Views.Networks.Visible = false
	gui.Views.Main.Visible = false
	gui.Views.Options.Visible = false
	gui.Views.Information.Visible = false
	gui.Views.AppStatus.Visible = false
	gui.Views.FilterPrefix.Visible = false
	gui.Views.Filter.Visible = false
}

// showAllPanels shows all regular UI panels
func (gui *Gui) showAllPanels() {
	gui.Views.ModeTabs.Visible = true
	gui.Views.Project.Visible = true
	gui.Views.Services.Visible = true
	gui.Views.Containers.Visible = true
	gui.Views.Images.Visible = true
	gui.Views.Volumes.Visible = true
	gui.Views.Networks.Visible = true
	gui.Views.Main.Visible = true
	gui.Views.Options.Visible = true
	gui.Views.Information.Visible = true
	gui.Views.AppStatus.Visible = true
	gui.Views.FilterPrefix.Visible = true
	gui.Views.Filter.Visible = true
}

// getAboutContent returns the formatted about content
func (gui *Gui) getAboutContent() string {
	var configBuf bytes.Buffer
	_ = yaml.NewEncoder(&configBuf, yaml.IncludeOmitted).Encode(gui.Config.UserConfig)

	return strings.Join(
		[]string{
			lazydockerTitle(),
			"Keybindings: https://github.com/peauc/lazydocker-ng/blob/master/docs/keybindings",
			"Config Options: https://github.com/peauc/lazydocker-ng/blob/master/docs/Config.md",
			"Raise an Issue: https://github.com/peauc/lazydocker-ng/issues",
			"Here's your lazydocker config when merged in with the defaults (you can open your config by pressing 'o'):",
			utils.ColoredYamlString(configBuf.String()),
			"Copyright (c) 2019 Jesse Duffield",
		}, "\n\n")
}

// renderAboutOptions shows the options bar at the bottom
func (gui *Gui) renderAboutOptions() error {
	optionsMap := map[string]string{
		"esc/q": gui.Tr.Close,
		"↑ ↓":   gui.Tr.Scroll,
		"o":     gui.Tr.OpenConfig,
	}
	return gui.renderOptionsMap(optionsMap)
}

// scrollUpAbout scrolls up in the About view
func (gui *Gui) scrollUpAbout(g *gocui.Gui, v *gocui.View) error {
	aboutView := gui.Views.About
	ox, oy := aboutView.Origin()
	newOy := int(math.Max(0, float64(oy-gui.Config.UserConfig.Gui.ScrollHeight)))

	aboutView.SetOrigin(ox, newOy)
	return nil
}

// scrollDownAbout scrolls down in the About view
func (gui *Gui) scrollDownAbout(g *gocui.Gui, v *gocui.View) error {
	aboutView := gui.Views.About
	ox, oy := aboutView.Origin()

	aboutView.SetOrigin(ox, oy+gui.Config.UserConfig.Gui.ScrollHeight)
	return nil
}

// lazydockerTitle returns ASCII art
func lazydockerTitle() string {
	return `
  ██╗      █████╗ ███████╗██╗   ██╗██████╗  ██████╗  ██████╗██╗  ██╗███████╗██████╗
  ██║     ██╔══██╗╚══███╔╝╚██╗ ██╔╝██╔══██╗██╔═══██╗██╔════╝██║ ██╔╝██╔════╝██╔══██╗
  ██║     ███████║  ███╔╝  ╚████╔╝ ██║  ██║██║   ██║██║     █████╔╝ █████╗  ██████╔╝
  ██║     ██╔══██║ ███╔╝    ╚██╔╝  ██║  ██║██║   ██║██║     ██╔═██╗ ██╔══╝  ██╔══██╗
  ███████╗██║  ██║███████╗   ██║   ██████╔╝╚██████╔╝╚██████╗██║  ██╗███████╗██║  ██║
  ╚══════╝╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═════╝  ╚═════╝  ╚═════╝╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝

                           ███╗   ██╗ ██████╗
                           ████╗  ██║██╔════╝
                           ██╔██╗ ██║██║  ███╗
                           ██║╚██╗██║██║   ██║
                           ██║ ╚████║╚██████╔╝
                           ╚═╝  ╚═══╝ ╚═════╝
`
}
