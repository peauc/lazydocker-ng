package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConfigHome pins the folder that lazydocker-ng (and upstream lazydocker)
// has always resolved through OpenPeeDeeP/xdg: XDG_CONFIG_HOME wins on every
// OS, otherwise each OS has its own default.
func TestConfigHome(t *testing.T) {
	scenarios := []struct {
		goos string
		env  map[string]string
		want string
	}{
		{"linux", map[string]string{"HOME": "/home/u"}, filepath.Join("/home/u", ".config")},
		{"linux", map[string]string{"HOME": "/home/u", "XDG_CONFIG_HOME": "/x"}, "/x"},
		{"darwin", map[string]string{"HOME": "/Users/u"}, filepath.Join("/Users/u", "Library", "Application Support")},
		{"darwin", map[string]string{"HOME": "/Users/u", "XDG_CONFIG_HOME": "/x"}, "/x"},
		{"windows", map[string]string{"APPDATA": `C:\Users\u\AppData\Roaming`}, `C:\Users\u\AppData\Roaming`},
		{"windows", map[string]string{"APPDATA": `C:\Users\u\AppData\Roaming`, "XDG_CONFIG_HOME": "/x"}, "/x"},
		{"freebsd", map[string]string{"HOME": "/home/u"}, filepath.Join("/home/u", ".config")},
	}

	for _, s := range scenarios {
		getenv := func(key string) string { return s.env[key] }
		assert.Equal(t, s.want, configHome(s.goos, getenv), "goos=%s env=%v", s.goos, s.env)
	}
}
