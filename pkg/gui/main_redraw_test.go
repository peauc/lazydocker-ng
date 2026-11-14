package gui

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/jesseduffield/gocui"
	"github.com/peauc/lazydocker-ng/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func newGuiWithMainView(t *testing.T) (*Gui, *atomic.Int32) {
	t.Helper()

	g, err := gocui.NewGui(gocui.NewGuiOpts{
		OutputMode:       gocui.OutputTrue,
		RuneReplacements: map[rune]string{},
		Headless:         true,
	})
	assert.NoError(t, err)
	t.Cleanup(g.Close)

	gui := &Gui{g: g}
	gui.Views.Main, err = g.SetView("main", 0, 0, 80, 20, 0)
	if err != nil && err.Error() != UnknownViewErrorMsg {
		t.Fatal(err)
	}

	redraws := &atomic.Int32{}
	gui.mainRedraw = utils.NewThrottle(time.Millisecond, func() { redraws.Add(1) })
	t.Cleanup(gui.mainRedraw.Stop)

	return gui, redraws
}

func TestMainViewWriterAsksForRedraw(t *testing.T) {
	gui, redraws := newGuiWithMainView(t)

	n, err := mainViewWriter{gui: gui}.Write([]byte("a log line\n"))
	assert.NoError(t, err)
	assert.Equal(t, 11, n)
	assert.Contains(t, gui.Views.Main.Buffer(), "a log line")

	assert.Eventually(t, func() bool { return redraws.Load() == 1 }, time.Second, time.Millisecond)
}

func TestClearMainViewAsksForRedraw(t *testing.T) {
	gui, redraws := newGuiWithMainView(t)

	gui.clearMainView()

	assert.Eventually(t, func() bool { return redraws.Load() == 1 }, time.Second, time.Millisecond)
}

func TestNoRedrawWithoutWrites(t *testing.T) {
	_, redraws := newGuiWithMainView(t)

	time.Sleep(20 * time.Millisecond)

	assert.Zero(t, redraws.Load())
}

func TestRequestMainRedrawBeforeRun(t *testing.T) {
	// SetupFakeGui and early start-up have no throttle yet
	assert.NotPanics(t, func() { (&Gui{}).requestMainRedraw() })
}
