package config

import (
	"path/filepath"
	"testing"

	"github.com/jesseduffield/yaml"
	"github.com/stretchr/testify/assert"
)

func loadConfigString(t *testing.T, content string) *UserConfig {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	writeFile(t, path, content)
	base := GetDefaultConfig()
	cfg, _, err := loadUserConfig(path, &base)
	if err != nil {
		t.Fatalf("loadUserConfig(%q): %v", content, err)
	}
	return cfg
}

func TestLegacyKeys(t *testing.T) {
	openCommand := func(c *UserConfig) any { return c.OS.OpenCommand }
	openLinkCommand := func(c *UserConfig) any { return c.OS.OpenLinkCommand }
	viewAllLogs := func(c *UserConfig) any { return c.CommandTemplates.ViewAllLogs }
	execShells := func(c *UserConfig) any { return c.CommandTemplates.PreferredExecShells }
	ignoreMouse := func(c *UserConfig) any { return c.Gui.IgnoreMouseEvents }

	scenarios := []struct {
		name    string
		content string
		got     func(*UserConfig) any
		want    any
	}{
		{"oS still works", "oS:\n  openCommand: foo\n", openCommand, "foo"},
		{"oS merges with defaults", "oS:\n  openCommand: foo\n", openLinkCommand, GetPlatformDefaultConfig().OpenLinkCommand},
		{"os wins over oS", "os:\n  openCommand: bar\noS:\n  openCommand: foo\n", openCommand, "bar"},
		{"viewAlLogs still works", "commandTemplates:\n  viewAlLogs: old\n", viewAllLogs, "old"},
		{"viewAllLogs wins over viewAlLogs", "commandTemplates:\n  viewAllLogs: new\n  viewAlLogs: old\n", viewAllLogs, "new"},
		{"preferedExecShell still works", "commandTemplates:\n  preferedExecShell: [zsh]\n", execShells, []string{"zsh"}},
		{"preferredExecShell wins", "commandTemplates:\n  preferredExecShell: [fish]\n  preferedExecShell: [zsh]\n", execShells, []string{"fish"}},
		{"mouseEvents still works", "gui:\n  mouseEvents: true\n", ignoreMouse, true},
		{"ignoreMouseEvents wins over mouseEvents", "gui:\n  ignoreMouseEvents: false\n  mouseEvents: true\n", ignoreMouse, false},
		{"ignoreMouseEvents works", "gui:\n  ignoreMouseEvents: true\n", ignoreMouse, true},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			assert.Equal(t, s.want, s.got(loadConfigString(t, s.content)))
		})
	}
}

func TestDefaultConfigUsesCorrectedSpellings(t *testing.T) {
	out, err := yaml.Marshal(GetDefaultConfig())
	assert.NoError(t, err)
	assert.Contains(t, string(out), "viewAllLogs:")
	assert.Contains(t, string(out), "\nos:")
	assert.NotContains(t, string(out), "viewAlLogs")
	assert.NotContains(t, string(out), "oS:")
}
