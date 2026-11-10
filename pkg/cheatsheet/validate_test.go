package cheatsheet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func writeCheatsheet(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestChangedCheatsheets(t *testing.T) {
	expected, actual := t.TempDir(), t.TempDir()
	writeCheatsheet(t, expected, "Keybindings_en.md", "same")
	writeCheatsheet(t, actual, "Keybindings_en.md", "same")
	writeCheatsheet(t, expected, "Keybindings_de.md", "new")
	writeCheatsheet(t, actual, "Keybindings_de.md", "old")
	writeCheatsheet(t, expected, "Keybindings_fr.md", "only expected")
	writeCheatsheet(t, actual, "Keybindings_nl.md", "only actual")
	writeCheatsheet(t, actual, "README.md", "not a cheatsheet")

	changed, err := changedCheatsheets(expected, actual)

	assert.NoError(t, err)
	assert.Equal(t, []string{"Keybindings_de.md", "Keybindings_fr.md", "Keybindings_nl.md"}, changed)
}
