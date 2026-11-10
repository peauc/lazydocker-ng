package i18n

import "strings"

// languageFromLocale returns the language part of a locale such as
// "fr_FR.UTF-8" or "pt-BR" ("fr", "pt"), the way lazydocker has always
// read it.
func languageFromLocale(locale string) string {
	locale, _, _ = strings.Cut(locale, ".")
	language, _, _ := strings.Cut(strings.ReplaceAll(locale, "-", "_"), "_")
	return language
}
