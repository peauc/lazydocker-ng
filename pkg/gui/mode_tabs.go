package gui

import "github.com/jesseduffield/gocui"

func (gui *Gui) handleToggleMode(g *gocui.Gui, v *gocui.View) error {
	return gui.toggleMode()
}

func (gui *Gui) updateModeTabsView() {
	gui.g.Update(func(*gocui.Gui) error {
		if gui.Views.ModeTabs != nil {
			gui.Views.ModeTabs.TabIndex = int(gui.State.UIMode)
		}
		return nil
	})
}
