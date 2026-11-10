package config

import (
	"path/filepath"
	"testing"

	"github.com/jesseduffield/yaml"
	"github.com/stretchr/testify/assert"
)

func parseDoc(t *testing.T, content string) yaml.MapSlice {
	t.Helper()
	var doc yaml.MapSlice
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		t.Fatalf("parse %q: %v", content, err)
	}
	return doc
}

func TestUnknownKeysReportsPaths(t *testing.T) {
	doc := parseDoc(t, "gui:\n  scrollHieght: 3\ncustomCommands:\n  containers:\n    - name: x\n      comand: y\nbogus: 1\n")

	assert.Equal(t, []string{"gui.scrollHieght", "customCommands.containers[0].comand", "bogus"}, unknownKeys(doc))
}

func TestUnknownKeysAcceptsDefaultConfig(t *testing.T) {
	raw, err := yaml.Marshal(GetDefaultConfig())
	assert.NoError(t, err)

	assert.Empty(t, unknownKeys(parseDoc(t, string(raw))))
}

func TestUnknownKeysAcceptsLegacyAndMapKeys(t *testing.T) {
	doc := parseDoc(t, "oS: {}\ngui: {mouseEvents: true}\ncommandTemplates:\n  viewAlLogs: x\nreplacements:\n  imageNamePrefixes:\n    anything: x\n")

	assert.Empty(t, unknownKeys(doc))
}

func TestNewAppConfigCollectsWarnings(t *testing.T) {
	dir := isolateConfigDir(t)
	path := filepath.Join(dir, "config.yml")
	writeFile(t, path, "gui:\n  bogus: 1\n")

	ac := newAppConfigForTest(t)

	assert.Equal(t, []string{path + `: unknown key "gui.bogus" (ignored)`}, ac.Warnings)
}

func TestNewAppConfigEmptyOrCommentOnly(t *testing.T) {
	for name, content := range map[string]string{
		"empty":        "",
		"comment only": "# only a comment\n",
		"bom and crlf": "\xEF\xBB\xBFgui:\r\n  scrollHeight: 7\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			dir := isolateConfigDir(t)
			writeFile(t, filepath.Join(dir, "config.yml"), content)

			ac := newAppConfigForTest(t)

			assert.Empty(t, ac.Warnings)
			if name == "bom and crlf" {
				assert.Equal(t, 7, ac.UserConfig.Gui.ScrollHeight)
			}
		})
	}
}
