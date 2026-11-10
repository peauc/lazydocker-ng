//go:build !windows

package i18n

import (
	"errors"
	"os"
)

// detectSystemLanguage reads the user's language from LC_ALL, then LANG.
func detectSystemLanguage() (string, error) {
	locale := os.Getenv("LC_ALL")
	if locale == "" {
		locale = os.Getenv("LANG")
	}
	if locale == "" {
		return "", errors.New("could not detect language: LC_ALL and LANG are empty")
	}
	return languageFromLocale(locale), nil
}
