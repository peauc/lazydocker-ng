package i18n

import (
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestTranslationSetFallsBackToEnglish(t *testing.T) {
	log := logrus.New()
	log.Out = io.Discard

	french := NewTranslationSet(logrus.NewEntry(log), "fr")

	assert.Equal(t, frenchSet().ErrorTitle, french.ErrorTitle, "translated keys use French")
	assert.Equal(t, englishSet().ConfigWarningsTitle, french.ConfigWarningsTitle, "missing keys fall back to English")
}
