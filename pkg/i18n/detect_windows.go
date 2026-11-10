//go:build windows

package i18n

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// localeNameMaxLength is LOCALE_NAME_MAX_LENGTH from winnls.h.
const localeNameMaxLength = 85

var kernel32 = windows.NewLazySystemDLL("kernel32.dll")

// detectSystemLanguage reads the user's default locale (their regional
// format), falling back to the system default locale, the way lazydocker has
// always done it on Windows.
func detectSystemLanguage() (string, error) {
	for _, proc := range []string{"GetUserDefaultLocaleName", "GetSystemDefaultLocaleName"} {
		if locale, err := windowsLocale(proc); err == nil && locale != "" {
			return languageFromLocale(locale), nil
		}
	}
	return "", errors.New("could not detect language: no Windows locale name")
}

// windowsLocale calls GetUserDefaultLocaleName or GetSystemDefaultLocaleName,
// which share the signature (LPWSTR lpLocaleName, int cchLocaleName).
func windowsLocale(procName string) (string, error) {
	buffer := make([]uint16, localeNameMaxLength)
	r, _, err := kernel32.NewProc(procName).Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer))) //nolint:gosec // the Win32 call needs the buffer's address
	if r == 0 {
		return "", err
	}
	return windows.UTF16ToString(buffer), nil
}
