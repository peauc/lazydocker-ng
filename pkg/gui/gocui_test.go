package gui

import (
	"testing"

	"github.com/jesseduffield/gocui"
	"github.com/stretchr/testify/assert"
)

func TestGetGocuiAttributeHex(t *testing.T) {
	assert.Equal(t, gocui.NewRGBColor(255, 128, 0), GetGocuiAttribute("#ff8000"))
	assert.Equal(t, gocui.NewRGBColor(0, 255, 0), GetGocuiAttribute("#0f0"))
	assert.Equal(t, gocui.NewRGBColor(170, 187, 204), GetGocuiAttribute("#ABC"))
	assert.Equal(t, gocui.ColorGreen, GetGocuiAttribute("green"))
	assert.Equal(t, gocui.ColorDefault, GetGocuiAttribute("not-a-colour"))
}
