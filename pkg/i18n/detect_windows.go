//go:build windows

package i18n

import (
	"errors"

	"golang.org/x/sys/windows"
)

// detectSystemLanguage reads the user's preferred Windows UI language.
func detectSystemLanguage() (string, error) {
	languages, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil {
		return "", err
	}
	if len(languages) == 0 {
		return "", errors.New("could not detect language: no preferred UI language")
	}
	return languageFromLocale(languages[0]), nil
}
