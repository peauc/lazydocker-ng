package gui

import (
	"github.com/jesseduffield/gocui"
	"github.com/peauc/lazydocker-ng/pkg/utils"
)

var gocuiColorMap = map[string]gocui.Attribute{
	"default":   gocui.ColorDefault,
	"black":     gocui.ColorBlack,
	"red":       gocui.ColorRed,
	"green":     gocui.ColorGreen,
	"yellow":    gocui.ColorYellow,
	"blue":      gocui.ColorBlue,
	"magenta":   gocui.ColorMagenta,
	"cyan":      gocui.ColorCyan,
	"white":     gocui.ColorWhite,
	"bold":      gocui.AttrBold,
	"reverse":   gocui.AttrReverse,
	"underline": gocui.AttrUnderline,
}

// GetAttribute gets the gocui color attribute from the string
func GetGocuiAttribute(key string) gocui.Attribute {
	if utils.IsValidHexValue(key) {
		r, g, b := hexToRGB(key)
		return gocui.NewRGBColor(r, g, b)
	}

	value, present := gocuiColorMap[key]
	if present {
		return value
	}
	return gocui.ColorDefault
}

// GetGocuiStyle bitwise OR's a list of attributes obtained via the given keys
func GetGocuiStyle(keys []string) gocui.Attribute {
	var attribute gocui.Attribute
	for _, key := range keys {
		attribute |= GetGocuiAttribute(key)
	}
	return attribute
}

// hexToRGB parses "#rgb" or "#rrggbb" (already checked by utils.IsValidHexValue)
// into its red, green and blue components; "#rgb" doubles each digit.
func hexToRGB(hex string) (r, g, b int32) {
	digits := hex[1:]
	if len(digits) == 3 {
		digits = string([]byte{digits[0], digits[0], digits[1], digits[1], digits[2], digits[2]})
	}
	component := func(i int) int32 { return hexDigit(digits[i])<<4 | hexDigit(digits[i+1]) }
	return component(0), component(2), component(4)
}

func hexDigit(c byte) int32 {
	switch {
	case c >= '0' && c <= '9':
		return int32(c - '0')
	case c >= 'a' && c <= 'f':
		return int32(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int32(c-'A') + 10
	}
	return 0
}
