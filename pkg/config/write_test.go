package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newIsolatedAppConfig(t *testing.T) *AppConfig {
	t.Helper()
	dir := isolateConfigDir(t)
	ac, err := NewAppConfig("lazydocker-ng", "version", "commit", "date", "buildSource", false, nil, "projectDir")
	if err != nil {
		t.Fatalf("NewAppConfig: %v", err)
	}
	if ac.ConfigDir != dir {
		t.Fatalf("ConfigDir = %q, want the isolated %q", ac.ConfigDir, dir)
	}
	return ac
}

func TestWriteToUserConfigTruncatesShorterContent(t *testing.T) {
	ac := newIsolatedAppConfig(t)

	err := ac.WriteToUserConfig(func(uc *UserConfig) error {
		uc.ConfirmOnQuit = true
		uc.Logs.Since = "a-long-value-to-grow-the-file"
		return nil
	})
	assert.NoError(t, err)

	err = ac.WriteToUserConfig(func(uc *UserConfig) error {
		uc.ConfirmOnQuit = false
		uc.Logs.Since = ""
		return nil
	})
	assert.NoError(t, err)

	raw, err := os.ReadFile(ac.ConfigFilename())
	assert.NoError(t, err)
	assert.Equal(t, "{}\n", string(raw))
}

func TestWriteToUserConfigLeavesNoTempFiles(t *testing.T) {
	ac := newIsolatedAppConfig(t)

	err := ac.WriteToUserConfig(func(uc *UserConfig) error {
		uc.ConfirmOnQuit = true
		return nil
	})
	assert.NoError(t, err)

	entries, err := os.ReadDir(ac.ConfigDir)
	assert.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	assert.Equal(t, []string{"config.yml"}, names)
}

func TestWriteToUserConfigUpdateErrorKeepsFile(t *testing.T) {
	ac := newIsolatedAppConfig(t)
	original := []byte("confirmOnQuit: true\n")
	if err := os.WriteFile(ac.ConfigFilename(), original, 0o644); err != nil {
		t.Fatal(err)
	}

	updateErr := errors.New("update failed")
	err := ac.WriteToUserConfig(func(uc *UserConfig) error { return updateErr })
	assert.ErrorIs(t, err, updateErr)

	raw, err := os.ReadFile(ac.ConfigFilename())
	assert.NoError(t, err)
	assert.Equal(t, original, raw)
}

func TestWriteFileAtomicKeepsMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file modes")
	}
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("old\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}

	assert.NoError(t, writeFileAtomic(path, []byte("new\n")))

	info, err := os.Stat(path)
	assert.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
}

func TestWriteFileAtomicFollowsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "real.yml")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "config.yml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	assert.NoError(t, writeFileAtomic(link, []byte("new\n")))

	info, err := os.Lstat(link)
	assert.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0, "config.yml should still be a symlink")
	raw, err := os.ReadFile(target)
	assert.NoError(t, err)
	assert.Equal(t, "new\n", string(raw))
}
