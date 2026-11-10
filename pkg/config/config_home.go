package config

import "path/filepath"

// configHome returns the base folder for user config files, resolved the way
// lazydocker has always done it (formerly through OpenPeeDeeP/xdg), so config
// paths never move: XDG_CONFIG_HOME on every OS when set, otherwise
// ~/Library/Application Support on macOS, %APPDATA% on Windows and ~/.config
// everywhere else.
func configHome(goos string, getenv func(string) string) string {
	if dir := getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}

	switch goos {
	case "darwin":
		return filepath.Join(getenv("HOME"), "Library", "Application Support")
	case "windows":
		return getenv("APPDATA")
	default:
		return filepath.Join(getenv("HOME"), ".config")
	}
}
