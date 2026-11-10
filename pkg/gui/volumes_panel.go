package gui

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/jesseduffield/gocui"
	"github.com/peauc/lazydocker-ng/pkg/commands"
	"github.com/peauc/lazydocker-ng/pkg/config"
	"github.com/peauc/lazydocker-ng/pkg/gui/panels"
	"github.com/peauc/lazydocker-ng/pkg/gui/presentation"
	"github.com/peauc/lazydocker-ng/pkg/gui/types"
	"github.com/peauc/lazydocker-ng/pkg/tasks"
	"github.com/peauc/lazydocker-ng/pkg/utils"
	"github.com/samber/lo"
)

func (gui *Gui) getVolumesPanel() *panels.SideListPanel[*commands.Volume] {
	return &panels.SideListPanel[*commands.Volume]{
		ContextState: &panels.ContextState[*commands.Volume]{
			GetMainTabs: func() []panels.MainTab[*commands.Volume] {
				return []panels.MainTab[*commands.Volume]{
					{
						Key:    "config",
						Title:  gui.Tr.ConfigTitle,
						Render: gui.renderVolumeConfig,
					},
				}
			},
			GetItemContextCacheKey: func(volume *commands.Volume) string {
				return "volumes-" + volume.Name
			},
		},
		ListPanel: panels.ListPanel[*commands.Volume]{
			List: panels.NewFilteredList[*commands.Volume](),
			View: gui.Views.Volumes,
		},
		NoItemsMessage: gui.Tr.NoVolumes,
		Gui:            gui.intoInterface(),
		// we're sorting these volumes based on whether they have labels defined,
		// because those are the ones you typically care about.
		// Within that, we also sort them alphabetically
		Sort: func(a *commands.Volume, b *commands.Volume) bool {
			if len(a.Volume.Labels) == 0 && len(b.Volume.Labels) > 0 {
				return false
			}
			if len(a.Volume.Labels) > 0 && len(b.Volume.Labels) == 0 {
				return true
			}
			return a.Name < b.Name
		},
		GetTableCells: presentation.GetVolumeDisplayStrings,
		Hide: func() bool {
			return gui.State.UIMode != MODE_RESSOURCES
		},
	}
}

func (gui *Gui) renderVolumeConfig(volume *commands.Volume) tasks.TaskFunc {
	return gui.NewSimpleRenderStringTask(func() string { return gui.volumeConfigStr(volume) })
}

func (gui *Gui) volumeConfigStr(volume *commands.Volume) string {
	padding := 15
	var output strings.Builder
	output.WriteString(utils.WithPadding("Name: ", padding) + volume.Name + "\n")
	output.WriteString(utils.WithPadding("Driver: ", padding) + volume.Volume.Driver + "\n")
	output.WriteString(utils.WithPadding("Scope: ", padding) + volume.Volume.Scope + "\n")
	output.WriteString(utils.WithPadding("Mountpoint: ", padding) + volume.Volume.Mountpoint + "\n")
	output.WriteString(utils.WithPadding("Labels: ", padding) + utils.FormatMap(padding, volume.Volume.Labels) + "\n")
	output.WriteString(utils.WithPadding("Options: ", padding) + utils.FormatMap(padding, volume.Volume.Options) + "\n")

	output.WriteString(utils.WithPadding("Status: ", padding))
	if volume.Volume.Status != nil {
		output.WriteString("\n")
		for k, v := range volume.Volume.Status {
			output.WriteString(utils.FormatMapItem(padding, k, v))
		}
	} else {
		output.WriteString("n/a")
	}

	if volume.Volume.UsageData != nil {
		output.WriteString(utils.WithPadding("RefCount: ", padding) + fmt.Sprintf("%d", volume.Volume.UsageData.RefCount) + "\n")
		output.WriteString(utils.WithPadding("Size: ", padding) + utils.FormatBinaryBytes(int(volume.Volume.UsageData.Size)) + "\n")
	}

	return output.String()
}

func (gui *Gui) reloadVolumes() error {
	if err := gui.refreshStateVolumes(); err != nil {
		return err
	}

	return gui.Panels.Volumes.RerenderList()
}

func (gui *Gui) refreshStateVolumes() error {
	volumes, err := gui.DockerCommand.RefreshVolumes()
	if err != nil {
		return err
	}

	gui.Panels.Volumes.SetItems(volumes)

	return nil
}

func (gui *Gui) handleVolumesRemoveMenu(g *gocui.Gui, v *gocui.View) error {
	volume, err := gui.Panels.Volumes.GetSelectedItem()
	if err != nil {
		return nil
	}

	type removeVolumeOption struct {
		description string
		command     string
		force       bool
	}

	options := []*removeVolumeOption{
		{
			description: gui.Tr.Remove,
			command:     utils.WithShortSha("docker volume rm " + volume.Name),
			force:       false,
		},
		{
			description: gui.Tr.ForceRemove,
			command:     utils.WithShortSha("docker volume rm --force " + volume.Name),
			force:       true,
		},
	}

	menuItems := lo.Map(options, func(option *removeVolumeOption, _ int) *types.MenuItem {
		return &types.MenuItem{
			LabelColumns: []string{option.description, color.New(color.FgRed).Sprint(option.command)},
			OnPress: func() error {
				return gui.WithWaitingStatus(gui.Tr.RemovingStatus, func() error {
					if err := volume.Remove(option.force); err != nil {
						return gui.createErrorPanel(err.Error())
					}
					return nil
				})
			},
		}
	})

	return gui.Menu(CreateMenuOptions{
		Title: "",
		Items: menuItems,
	})
}

func (gui *Gui) handlePruneVolumes() error {
	return gui.createConfirmationPanel(gui.Tr.Confirm, gui.Tr.ConfirmPruneVolumes, func(g *gocui.Gui, v *gocui.View) error {
		return gui.WithWaitingStatus(gui.Tr.PruningStatus, func() error {
			err := gui.DockerCommand.PruneVolumes()
			if err != nil {
				return gui.createErrorPanel(err.Error())
			}
			return nil
		})
	}, nil)
}

func (gui *Gui) handleVolumesCustomCommand(g *gocui.Gui, v *gocui.View) error {
	volume, err := gui.Panels.Volumes.GetSelectedItem()
	if err != nil {
		return nil
	}

	commandObject := gui.DockerCommand.NewCommandObject(commands.CommandObject{
		Volume: volume,
	})

	customCommands := gui.Config.UserConfig.CustomCommands.Volumes

	return gui.createCustomCommandMenu(customCommands, commandObject)
}

func (gui *Gui) handleVolumesBulkCommand(g *gocui.Gui, v *gocui.View) error {
	baseBulkCommands := make([]config.CustomCommand, 0, 1+len(gui.Config.UserConfig.BulkCommands.Volumes))
	baseBulkCommands = append(baseBulkCommands, config.CustomCommand{
		Name:             gui.Tr.PruneVolumes,
		InternalFunction: gui.handlePruneVolumes,
	})

	bulkCommands := append(baseBulkCommands, gui.Config.UserConfig.BulkCommands.Volumes...)
	commandObject := gui.DockerCommand.NewCommandObject(commands.CommandObject{})

	return gui.createBulkCommandMenu(bulkCommands, commandObject)
}
