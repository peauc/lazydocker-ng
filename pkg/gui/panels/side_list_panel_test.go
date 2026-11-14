package panels

import (
	"context"
	"strings"
	"testing"

	"github.com/jesseduffield/gocui"
	"github.com/peauc/lazydocker-ng/pkg/tasks"
	"github.com/stretchr/testify/assert"
)

// fakeGui runs updates straight away and counts them.
type fakeGui struct {
	filterString  string
	ignoreStrings []string
	updates       int
}

func (g *fakeGui) HandleClick(*gocui.View, int, *int, func() error) error { return nil }

func (g *fakeGui) NewSimpleRenderStringTask(func() string) tasks.TaskFunc {
	return func(context.Context) {}
}
func (g *fakeGui) FocusY(int, int, *gocui.View)              {}
func (g *fakeGui) ShouldRefresh(string) bool                 { return false }
func (g *fakeGui) GetMainView() *gocui.View                  { return nil }
func (g *fakeGui) IsCurrentView(*gocui.View) bool            { return false }
func (g *fakeGui) FilterString(*gocui.View) string           { return g.filterString }
func (g *fakeGui) IgnoreStrings() []string                   { return g.ignoreStrings }
func (g *fakeGui) QueueTask(func(ctx context.Context)) error { return nil }

func (g *fakeGui) Update(f func() error) {
	g.updates++
	_ = f()
}

type testItem struct {
	name string
	cpu  string
}

type testPanel struct {
	*SideListPanel[*testItem]
	gui       *fakeGui
	cellCalls map[string]int
	hidden    bool
}

func newTestPanel(t *testing.T) *testPanel {
	t.Helper()

	g, err := gocui.NewGui(gocui.NewGuiOpts{
		OutputMode:       gocui.OutputTrue,
		RuneReplacements: map[rune]string{},
		Headless:         true,
	})
	assert.NoError(t, err)
	t.Cleanup(g.Close)

	view, err := g.SetView("list", 0, 0, 80, 20, 0)
	if err != nil && err.Error() != "unknown view" {
		assert.NoError(t, err)
	}

	panel := &testPanel{gui: &fakeGui{}, cellCalls: map[string]int{}}
	panel.SideListPanel = &SideListPanel[*testItem]{
		ListPanel: ListPanel[*testItem]{
			List: NewFilteredList[*testItem](),
			View: view,
		},
		Gui:  panel.gui,
		Sort: func(a, b *testItem) bool { return a.name < b.name },
		GetTableCells: func(item *testItem) []string {
			panel.cellCalls[item.name]++
			return []string{item.name, item.cpu}
		},
		Hide: func() bool { return panel.hidden },
	}
	return panel
}

func (p *testPanel) shown() string {
	return strings.TrimRight(p.View.Buffer(), "\n")
}

func TestFilterAndSortBuildsCellsOncePerItem(t *testing.T) {
	panel := newTestPanel(t)
	panel.gui.ignoreStrings = []string{"nope", "hidden", "never"}
	panel.gui.filterString = "a"
	panel.SetItems([]*testItem{{name: "banana"}, {name: "apple"}, {name: "hidden-a"}, {name: "kiwi"}})

	for name, calls := range panel.cellCalls {
		assert.Equal(t, 1, calls, name)
	}

	items := panel.List.GetItems()
	assert.Len(t, items, 2)
	assert.Equal(t, "apple", items[0].name)
	assert.Equal(t, "banana", items[1].name)
}

func TestFilterAndSortSkipsCellsWithoutFilters(t *testing.T) {
	panel := newTestPanel(t)
	panel.SetItems([]*testItem{{name: "b"}, {name: "a"}})

	assert.Empty(t, panel.cellCalls)
	assert.Equal(t, 2, panel.List.Len())
}

func TestRerenderListIfChanged(t *testing.T) {
	panel := newTestPanel(t)
	item := &testItem{name: "web", cpu: "1.00%"}
	panel.SetItems([]*testItem{item})

	assert.NoError(t, panel.RerenderListIfChanged())
	assert.Equal(t, 1, panel.gui.updates, "the first render draws")
	assert.Equal(t, "web 1.00%", panel.shown())

	assert.NoError(t, panel.RerenderListIfChanged())
	assert.Equal(t, 1, panel.gui.updates, "an unchanged list is not redrawn")

	item.cpu = "2.00%"
	assert.NoError(t, panel.RerenderListIfChanged())
	assert.Equal(t, 2, panel.gui.updates, "a changed list is redrawn")
	assert.Equal(t, "web 2.00%", panel.shown())

	panel.hidden = true
	item.cpu = "3.00%"
	assert.NoError(t, panel.RerenderListIfChanged())
	assert.Equal(t, 2, panel.gui.updates, "a hidden list is left alone")

	panel.hidden = false
	assert.NoError(t, panel.RerenderListIfChanged())
	assert.Equal(t, "web 3.00%", panel.shown(), "it catches up once visible again")
}

func TestRerenderListAlwaysDraws(t *testing.T) {
	panel := newTestPanel(t)
	panel.SetItems([]*testItem{{name: "web", cpu: "1.00%"}})

	assert.NoError(t, panel.RerenderList())
	assert.NoError(t, panel.RerenderList())
	assert.Equal(t, 2, panel.gui.updates)

	// what RerenderList drew counts as shown for the polling path
	assert.NoError(t, panel.RerenderListIfChanged())
	assert.Equal(t, 2, panel.gui.updates)

	panel.SetItems(nil)
	assert.NoError(t, panel.RerenderListIfChanged())
	assert.Equal(t, 3, panel.gui.updates)
	assert.Equal(t, "", panel.shown())
}
