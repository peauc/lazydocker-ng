package cheatsheet

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/jesseduffield/lazycore/pkg/utils"
)

var cheatsheetFile = regexp.MustCompile(`^Keybindings_\w+\.md$`)

func Check() {
	changed, err := checkCheatsheets()
	if err != nil {
		log.Fatalf("Error occurred while checking if cheatsheets are up to date: %v", err)
	}

	if len(changed) > 0 {
		fmt.Println("These cheatsheets are out of date:")
		for _, name := range changed {
			fmt.Println("  " + name)
		}
		fmt.Printf(
			"\nCheatsheets are out of date. Please run `%s` at the project root and commit the changes. "+
				"If you run the script and no keybindings files are updated as a result, try rebasing onto master"+
				"and trying again.\n",
			generateCheatsheetCmd,
		)
		os.Exit(1)
	}

	fmt.Println("\nCheatsheets are up to date")
}

// checkCheatsheets generates the cheatsheets into a temp dir and returns the
// ones that differ from docs/keybindings.
func checkCheatsheets() ([]string, error) {
	tmpDir, err := os.MkdirTemp("", "lazydocker_cheatsheet")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	generateAtDir(tmpDir)

	return changedCheatsheets(tmpDir, GetKeybindingsDir())
}

func GetKeybindingsDir() string {
	return utils.GetLazyRootDirectory() + "/docs/keybindings"
}

// changedCheatsheets returns, sorted, the Keybindings_*.md files that differ
// between expectedDir and actualDir or exist in only one of them.
func changedCheatsheets(expectedDir, actualDir string) ([]string, error) {
	expected, err := readCheatsheets(expectedDir)
	if err != nil {
		return nil, err
	}
	if len(expected) == 0 {
		return nil, errors.New("no cheatsheets were generated")
	}
	actual, err := readCheatsheets(actualDir)
	if err != nil {
		return nil, err
	}

	changed := []string{}
	for name, content := range expected {
		if other, ok := actual[name]; !ok || !bytes.Equal(content, other) {
			changed = append(changed, name)
		}
	}
	for name := range actual {
		if _, ok := expected[name]; !ok {
			changed = append(changed, name)
		}
	}
	slices.Sort(changed)
	return changed, nil
}

func readCheatsheets(dir string) (map[string][]byte, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := map[string][]byte{}
	for _, entry := range entries {
		if entry.IsDir() || !cheatsheetFile.MatchString(entry.Name()) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		files[entry.Name()] = content
	}
	return files, nil
}
