package i18n

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLanguageFromLocale(t *testing.T) {
	for locale, want := range map[string]string{
		"fr_FR.UTF-8": "fr",
		"pt-BR":       "pt",
		"zh_CN":       "zh",
		"C.UTF-8":     "C",
		"en":          "en",
	} {
		assert.Equal(t, want, languageFromLocale(locale), locale)
	}
}

func TestDetectSystemLanguageEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows reads the OS UI language, not the environment")
	}

	t.Setenv("LC_ALL", "de_DE.UTF-8")
	t.Setenv("LANG", "es_ES.UTF-8")
	language, err := detectSystemLanguage()
	assert.NoError(t, err)
	assert.Equal(t, "de", language, "LC_ALL wins")

	t.Setenv("LC_ALL", "")
	language, err = detectSystemLanguage()
	assert.NoError(t, err)
	assert.Equal(t, "es", language, "then LANG")

	t.Setenv("LANG", "")
	_, err = detectSystemLanguage()
	assert.Error(t, err)
	assert.Equal(t, "C", detectLanguage(detectSystemLanguage), "no locale falls back to C")
}
