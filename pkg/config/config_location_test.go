package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// isolateHome points every config-home input at one temp dir. XDG_CONFIG_HOME
// wins on every OS, so the config folders resolve to <home>/<name>.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("HOME", home)
	t.Setenv("APPDATA", home)
	return home
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func scrollHeight(n string) string { return "gui:\n  scrollHeight: " + n + "\n" }

func newAppConfigForTest(t *testing.T) *AppConfig {
	t.Helper()
	ac, err := NewAppConfig("lazydocker-ng", "version", "commit", "date", "buildSource", false, nil, "projectDir")
	if err != nil {
		t.Fatalf("NewAppConfig: %v", err)
	}
	return ac
}

func TestNewAppConfigPrefersLazydockerNg(t *testing.T) {
	home := isolateHome(t)
	writeFile(t, filepath.Join(home, "lazydocker-ng", "config.yml"), scrollHeight("5"))
	writeFile(t, filepath.Join(home, "lazydocker", "config.yml"), scrollHeight("9"))

	ac := newAppConfigForTest(t)

	assert.Equal(t, 5, ac.UserConfig.Gui.ScrollHeight)
	assert.Equal(t, filepath.Join(home, "lazydocker-ng", "config.yml"), ac.UserConfigPath)
	assert.False(t, ac.UsingUpstreamConfig())
}

func TestNewAppConfigFallsBackToUpstream(t *testing.T) {
	home := isolateHome(t)
	writeFile(t, filepath.Join(home, "lazydocker", "config.yml"), scrollHeight("9"))

	ac := newAppConfigForTest(t)

	assert.Equal(t, 9, ac.UserConfig.Gui.ScrollHeight)
	assert.Equal(t, filepath.Join(home, "lazydocker", "config.yml"), ac.UserConfigPath)
	assert.Equal(t, filepath.Join(home, "lazydocker-ng"), ac.ConfigDir)
	assert.True(t, ac.UsingUpstreamConfig())
}

func TestNewAppConfigUpstreamPrecedence(t *testing.T) {
	home := isolateHome(t)
	writeFile(t, filepath.Join(home, "jesseduffield", "lazydocker", "config.yml"), scrollHeight("7"))
	writeFile(t, filepath.Join(home, "lazydocker", "config.yml"), scrollHeight("9"))

	ac := newAppConfigForTest(t)

	assert.Equal(t, 7, ac.UserConfig.Gui.ScrollHeight)
	assert.Equal(t, filepath.Join(home, "jesseduffield", "lazydocker", "config.yml"), ac.UserConfigPath)
}

func TestNewAppConfigNoConfigAnywhere(t *testing.T) {
	home := isolateHome(t)

	ac := newAppConfigForTest(t)

	want := filepath.Join(home, "lazydocker-ng", "config.yml")
	assert.Equal(t, want, ac.UserConfigPath)
	info, err := os.Stat(want)
	assert.NoError(t, err)
	if err == nil {
		assert.Equal(t, int64(0), info.Size())
	}
}

func TestNewAppConfigNgFolderWithoutConfigFile(t *testing.T) {
	home := isolateHome(t)
	writeFile(t, filepath.Join(home, "lazydocker-ng", "development.log"), "{}\n")
	writeFile(t, filepath.Join(home, "lazydocker", "config.yml"), scrollHeight("9"))

	ac := newAppConfigForTest(t)

	assert.Equal(t, 9, ac.UserConfig.Gui.ScrollHeight)
	assert.True(t, ac.UsingUpstreamConfig())
}

func TestNewAppConfigConfigDirEnv(t *testing.T) {
	home := isolateHome(t)
	other := filepath.Join(home, "other")
	writeFile(t, filepath.Join(other, "config.yml"), scrollHeight("3"))
	writeFile(t, filepath.Join(home, "lazydocker-ng", "config.yml"), scrollHeight("5"))
	writeFile(t, filepath.Join(home, "lazydocker", "config.yml"), scrollHeight("9"))
	t.Setenv("CONFIG_DIR", other)

	ac := newAppConfigForTest(t)

	assert.Equal(t, 3, ac.UserConfig.Gui.ScrollHeight)
	assert.Equal(t, other, ac.ConfigDir)
	assert.False(t, ac.UsingUpstreamConfig())
}

type fileState struct {
	size    int64
	modTime time.Time
	content string
}

// snapshotDir records every entry of dir (the upstream folders in these tests
// are flat; a new subfolder shows up as an entry with no content).
func snapshotDir(t *testing.T, dir string) map[string]fileState {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]fileState{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		state := fileState{size: info.Size(), modTime: info.ModTime()}
		if !entry.IsDir() {
			content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			state.content = string(content)
		}
		files[entry.Name()] = state
	}
	return files
}

func TestNewAppConfigDoesNotTouchUpstreamFolder(t *testing.T) {
	home := isolateHome(t)
	upstream := filepath.Join(home, "lazydocker")
	writeFile(t, filepath.Join(upstream, "config.yml"), "# keep me\n"+scrollHeight("9"))
	before := snapshotDir(t, upstream)

	ac := newAppConfigForTest(t)

	assert.True(t, ac.UsingUpstreamConfig())
	assert.Equal(t, before, snapshotDir(t, upstream))
	_, err := os.Stat(filepath.Join(home, "lazydocker-ng", "config.yml"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestEditableConfigFilenameCopiesUpstreamOnce(t *testing.T) {
	home := isolateHome(t)
	upstreamFile := filepath.Join(home, "lazydocker", "config.yml")
	original := "# keep me\n" + scrollHeight("9")
	writeFile(t, upstreamFile, original)
	ac := newAppConfigForTest(t)

	path, err := ac.EditableConfigFilename()

	ngFile := filepath.Join(home, "lazydocker-ng", "config.yml")
	assert.NoError(t, err)
	assert.Equal(t, ngFile, path)
	copied, err := os.ReadFile(ngFile)
	assert.NoError(t, err)
	assert.Equal(t, original, string(copied))
	upstreamNow, _ := os.ReadFile(upstreamFile)
	assert.Equal(t, original, string(upstreamNow))
	assert.False(t, ac.UsingUpstreamConfig())

	writeFile(t, ngFile, "x: 1\n")
	path, err = ac.EditableConfigFilename()
	assert.NoError(t, err)
	assert.Equal(t, ngFile, path)
	again, _ := os.ReadFile(ngFile)
	assert.Equal(t, "x: 1\n", string(again))
}

func TestWriteToUserConfigInFallbackWritesNg(t *testing.T) {
	home := isolateHome(t)
	upstreamFile := filepath.Join(home, "lazydocker", "config.yml")
	writeFile(t, upstreamFile, scrollHeight("9"))
	ac := newAppConfigForTest(t)

	err := ac.WriteToUserConfig(func(uc *UserConfig) error {
		uc.ConfirmOnQuit = true
		return nil
	})
	assert.NoError(t, err)

	written, _, err := loadUserConfig(filepath.Join(home, "lazydocker-ng", "config.yml"), &UserConfig{})
	assert.NoError(t, err)
	if written != nil {
		assert.Equal(t, 9, written.Gui.ScrollHeight)
		assert.True(t, written.ConfirmOnQuit)
	}
	upstreamNow, _ := os.ReadFile(upstreamFile)
	assert.Equal(t, scrollHeight("9"), string(upstreamNow))
}

// An empty lazydocker-ng config.yml is the app's own artifact (it creates one
// on a first run without any config), so it must not hide upstream's config.
func TestNewAppConfigEmptyNgConfigFallsBackToUpstream(t *testing.T) {
	home := isolateHome(t)
	ngFile := filepath.Join(home, "lazydocker-ng", "config.yml")
	writeFile(t, ngFile, "")
	upstream := "# keep me\n" + scrollHeight("9")
	writeFile(t, filepath.Join(home, "lazydocker", "config.yml"), upstream)

	ac := newAppConfigForTest(t)

	assert.Equal(t, 9, ac.UserConfig.Gui.ScrollHeight)
	assert.True(t, ac.UsingUpstreamConfig())

	path, err := ac.EditableConfigFilename()
	assert.NoError(t, err)
	assert.Equal(t, ngFile, path)
	copied, _ := os.ReadFile(ngFile)
	assert.Equal(t, upstream, string(copied), "the first edit copies upstream over the empty file")
}
