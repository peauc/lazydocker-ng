package config

import (
	"os"
	"path/filepath"
)

// upstreamConfigFolders are upstream lazydocker's config folders, in the order
// upstream lazydocker itself checks them.
var upstreamConfigFolders = [][]string{{"jesseduffield", "lazydocker"}, {"lazydocker"}}

// configLocation says which config.yml is read at startup and where writes go.
type configLocation struct {
	Dir    string // lazydocker-ng's own folder; writes, logs and edits go here
	Source string // the config.yml read at startup
}

// resolveConfigLocation picks the config to read: CONFIG_DIR when set, else
// lazydocker-ng's own config.yml, else upstream lazydocker's (read in place,
// never written), else lazydocker-ng's (created empty by the caller).
// Presence is checked on config.yml rather than on the folder, because the
// app creates its own folder for the debug log.
func resolveConfigLocation(projectName, goos string, getenv func(string) string) configLocation {
	if dir := getenv("CONFIG_DIR"); dir != "" {
		return configLocation{Dir: dir, Source: filepath.Join(dir, "config.yml")}
	}

	home := configHome(goos, getenv)
	own := filepath.Join(home, projectName)
	location := configLocation{Dir: own, Source: filepath.Join(own, "config.yml")}
	if isFile(location.Source) {
		return location
	}

	for _, folder := range upstreamConfigFolders {
		candidate := filepath.Join(append(append([]string{home}, folder...), "config.yml")...)
		if isFile(candidate) {
			location.Source = candidate
			return location
		}
	}

	return location
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
