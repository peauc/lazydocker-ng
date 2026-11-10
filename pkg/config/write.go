package config

import (
	"errors"
	"os"
	"path/filepath"
)

// writeFileAtomic replaces the file at path with data by writing a temp file
// next to it and renaming it into place, so readers never see a partial or
// stale-tailed file. A symlinked path is resolved first so the link survives
// (dotfiles setups), and an existing file keeps its permissions.
func writeFileAtomic(path string, data []byte) (err error) {
	if target, evalErr := filepath.EvalSymlinks(path); evalErr == nil {
		path = target
	}

	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.yml.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
