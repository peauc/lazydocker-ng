package panels

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/go-errors/errors"
	"github.com/jesseduffield/gocui"
	"github.com/peauc/lazydocker-ng/pkg/tasks"
	"github.com/peauc/lazydocker-ng/pkg/utils"
	"github.com/samber/lo"
)

type ISideListPanel interface {
	SetMainTabIndex(int)
	HandleSelect() error
	GetView() *gocui.View
	Refocus()
	RerenderList() error
	IsFilterDisabled() bool
	IsHidden() bool
	HandleNextLine() error
	HandlePrevLine() error
	HandleClick() error
	HandlePrevMainTab() error
	HandleNextMainTab() error
}

// list panel at the side of the screen that renders content to the main panel
type SideListPanel[T comparable] struct {
	ContextState *ContextState[T]

	ListPanel[T]

	// message to render in the main view if there are no items in the panel
	// and it has focus. Leave empty if you don't want to render anything
	NoItemsMessage string

	// a representation of the gui
	Gui IGui

	// this Filter is applied on top of additional default filters
	Filter func(T) bool
	Sort   func(a, b T) bool

	// a callback to invoke when the item is clicked
	OnClick func(T) error

	// returns the cells that we render to the view in a table format. The cells will
	// be rendered with padding.
	GetTableCells func(T) []string

	// function to be called after re-rendering list. Can be nil
	OnRerender func() error

	// set this to true if you don't want to allow manual filtering via '/'
	DisableFilter bool

	// This can be nil if you want to always show the panel
	Hide func() bool

	// the table that the view currently shows; only written on the UI thread
	shownTable atomic.Pointer[string]
}

var _ ISideListPanel = &SideListPanel[int]{}

type IGui interface {
	HandleClick(v *gocui.View, itemCount int, selectedLine *int, handleSelect func() error) error
	NewSimpleRenderStringTask(getContent func() string) tasks.TaskFunc
	FocusY(selectedLine int, itemCount int, view *gocui.View)
	ShouldRefresh(contextKey string) bool
	GetMainView() *gocui.View
	IsCurrentView(*gocui.View) bool
	FilterString(view *gocui.View) string
	IgnoreStrings() []string
	Update(func() error)

	QueueTask(f func(ctx context.Context)) error
}

func (self *SideListPanel[T]) HandleClick() error {
	itemCount := self.List.Len()
	handleSelect := self.HandleSelect
	selectedLine := &self.SelectedIdx

	if err := self.Gui.HandleClick(self.View, itemCount, selectedLine, handleSelect); err != nil {
		return err
	}

	if self.OnClick != nil {
		selectedItem, err := self.GetSelectedItem()
		if err == nil {
			return self.OnClick(selectedItem)
		}
	}

	return nil
}

func (self *SideListPanel[T]) GetView() *gocui.View {
	return self.View
}

func (self *SideListPanel[T]) HandleSelect() error {
	item, err := self.GetSelectedItem()
	if err != nil {
		if err.Error() != self.NoItemsMessage {
			return err
		}

		if self.NoItemsMessage != "" {
			self.Gui.NewSimpleRenderStringTask(func() string { return self.NoItemsMessage })
		}

		return nil
	}

	self.Refocus()

	return self.renderContext(item)
}

func (self *SideListPanel[T]) renderContext(item T) error {
	if self.ContextState == nil {
		return nil
	}

	tabs := self.ContextState.GetMainTabs()
	if len(tabs) == 0 {
		return nil
	}

	key := self.ContextState.GetCurrentContextKey(item)
	if !self.Gui.ShouldRefresh(key) {
		return nil
	}

	mainView := self.Gui.GetMainView()
	mainView.Tabs = self.ContextState.GetMainTabTitles()
	mainView.TabIndex = self.ContextState.mainTabIdx

	task := self.ContextState.GetCurrentMainTab().Render(item)

	return self.Gui.QueueTask(task)
}

func (self *SideListPanel[T]) GetSelectedItem() (T, error) {
	var zero T

	item, ok := self.List.TryGet(self.SelectedIdx)
	if !ok {
		// could probably have a better error here
		return zero, errors.New(self.NoItemsMessage)
	}

	return item, nil
}

func (self *SideListPanel[T]) HandleNextLine() error {
	self.SelectNextLine()

	return self.HandleSelect()
}

func (self *SideListPanel[T]) HandlePrevLine() error {
	self.SelectPrevLine()

	return self.HandleSelect()
}

func (self *SideListPanel[T]) HandleNextMainTab() error {
	if self.ContextState == nil {
		return nil
	}

	self.ContextState.HandleNextMainTab()

	return self.HandleSelect()
}

func (self *SideListPanel[T]) HandlePrevMainTab() error {
	if self.ContextState == nil {
		return nil
	}

	self.ContextState.HandlePrevMainTab()

	return self.HandleSelect()
}

func (self *SideListPanel[T]) Refocus() {
	self.Gui.FocusY(self.SelectedIdx, self.List.Len(), self.View)
}

func (self *SideListPanel[T]) SetItems(items []T) {
	self.List.SetItems(items)
	self.FilterAndSort()
}

func (self *SideListPanel[T]) FilterAndSort() {
	filterString := self.Gui.FilterString(self.View)
	ignoreStrings := self.Gui.IgnoreStrings()

	self.List.Filter(func(item T, index int) bool {
		if self.Filter != nil && !self.Filter(item) {
			return false
		}

		if filterString == "" && len(ignoreStrings) == 0 {
			return true
		}

		// built once per item: the cells are not free to make
		cells := self.GetTableCells(item)
		anyCellContains := func(needle string) bool {
			return slices.ContainsFunc(cells, func(cell string) bool {
				return strings.Contains(cell, needle)
			})
		}

		if slices.ContainsFunc(ignoreStrings, anyCellContains) {
			return false
		}

		return filterString == "" || anyCellContains(filterString)
	})

	self.List.Sort(self.Sort)

	self.clampSelectedLineIdx()
}

func (self *SideListPanel[T]) RerenderList() error {
	self.FilterAndSort()

	self.Gui.Update(func() error {
		renderedTable, err := self.renderTable()
		if err != nil {
			return err
		}
		return self.writeList(renderedTable)
	})

	return nil
}

// RerenderListIfChanged is RerenderList for callers that poll: it leaves a
// hidden panel alone, and when the list would look the same as it does now it
// returns without asking for a redraw of the screen.
func (self *SideListPanel[T]) RerenderListIfChanged() error {
	if self.IsHidden() {
		return nil
	}

	self.FilterAndSort()

	renderedTable, err := self.renderTable()
	if err != nil {
		return err
	}
	if shown := self.shownTable.Load(); shown != nil && *shown == renderedTable {
		return nil
	}

	self.Gui.Update(func() error {
		return self.writeList(renderedTable)
	})

	return nil
}

func (self *SideListPanel[T]) renderTable() (string, error) {
	table := lo.Map(self.List.GetItems(), func(item T, index int) []string {
		return self.GetTableCells(item)
	})
	return utils.RenderTable(table)
}

// writeList puts the rendered table in the view. It must run on the UI thread.
func (self *SideListPanel[T]) writeList(renderedTable string) error {
	self.View.Clear()
	fmt.Fprint(self.View, renderedTable)
	self.shownTable.Store(&renderedTable)

	if self.OnRerender != nil {
		if err := self.OnRerender(); err != nil {
			return err
		}
	}

	if self.Gui.IsCurrentView(self.View) {
		return self.HandleSelect()
	}
	return nil
}

func (self *SideListPanel[T]) SetMainTabIndex(index int) {
	if self.ContextState == nil {
		return
	}

	self.ContextState.SetMainTabIndex(index)
}

func (self *SideListPanel[T]) IsFilterDisabled() bool {
	return self.DisableFilter
}

func (self *SideListPanel[T]) IsHidden() bool {
	if self.Hide == nil {
		return false
	}

	return self.Hide()
}
