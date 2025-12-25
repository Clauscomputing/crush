package styles

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

func NewChallengerDeepTheme() *Theme {
	t := &Theme{
		Name:   "challenger-deep",
		IsDark: true,

		Primary:   ParseHex("#65b2ff"), // Blue (color12)
		Secondary: ParseHex("#63f2f1"), // Cyan (color14)
		Tertiary:  ParseHex("#ffe9aa"), // Yellow (color3)
		Accent:    ParseHex("#62d196"), // Green (color10)

		// Backgrounds
		BgBase:        color.Transparent,   // .background (transparent)
		BgBaseLighter: ParseHex("#100e23"), // .color0
		BgSubtle:      ParseHex("#565575"), // .color8
		BgOverlay:     Alpha(ParseHex("#1b182c"), 200), // .background (semi-transparent)

		// Foregrounds
		FgBase:      ParseHex("#e5e5e5"), // .foreground
		FgMuted:     ParseHex("#a6b3cc"), // .color15
		FgHalfMuted: ParseHex("#a6b3cc"), // .color15
		FgSubtle:    ParseHex("#565575"), // .color8
		FgSelected:  ParseHex("#fbfcfc"), // .cursorColor

		// Borders
		Border:      ParseHex("#565575"), // .color8
		BorderFocus: ParseHex("#c991e1"), // Magenta (color5)

		// Status
		Success: ParseHex("#95ffa4"), // color2
		Error:   ParseHex("#ff8080"), // color1
		Warning: ParseHex("#ffe9aa"), // color3
		Info:    ParseHex("#91ddff"), // color4

		// Colors
		White: ParseHex("#cbe3e7"),

		BlueLight: ParseHex("#65b2ff"),
		BlueDark:  ParseHex("#91ddff"),
		Blue:      ParseHex("#91ddff"),

		Yellow: ParseHex("#ffe9aa"),
		Citron: ParseHex("#ffb378"),

		Green:      ParseHex("#95ffa4"),
		GreenDark:  ParseHex("#62d196"),
		GreenLight: ParseHex("#aaffe4"),

		Red:      ParseHex("#ff8080"),
		RedDark:  ParseHex("#ff5458"),
		RedLight: ParseHex("#ff8080"),
		Cherry:   ParseHex("#ff5458"),
	}

	// Text selection.
	t.TextSelection = lipgloss.NewStyle().Foreground(ParseHex("#fbfcfc")).Background(ParseHex("#c991e1"))

	// LSP and MCP status.
	t.ItemOfflineIcon = lipgloss.NewStyle().Foreground(ParseHex("#565575")).SetString("●")
	t.ItemBusyIcon = t.ItemOfflineIcon.Foreground(ParseHex("#ffe9aa"))
	t.ItemErrorIcon = t.ItemOfflineIcon.Foreground(ParseHex("#ff8080"))
	t.ItemOnlineIcon = t.ItemOfflineIcon.Foreground(ParseHex("#95ffa4"))

	// Editor: Yolo Mode.
	t.YoloIconFocused = lipgloss.NewStyle().Foreground(ParseHex("#1b182c")).Background(ParseHex("#ffe9aa")).Bold(true).SetString(" ! ")
	t.YoloIconBlurred = t.YoloIconFocused.Foreground(ParseHex("#cbe3e7")).Background(ParseHex("#565575"))
	t.YoloDotsFocused = lipgloss.NewStyle().Foreground(ParseHex("#ffe9aa")).SetString(":::")
	t.YoloDotsBlurred = t.YoloDotsFocused.Foreground(ParseHex("#565575"))

	// oAuth Chooser.
	t.AuthBorderSelected = lipgloss.NewStyle().BorderForeground(ParseHex("#95ffa4"))
	t.AuthTextSelected = lipgloss.NewStyle().Foreground(ParseHex("#95ffa4"))
	t.AuthBorderUnselected = lipgloss.NewStyle().BorderForeground(ParseHex("#565575"))
	t.AuthTextUnselected = lipgloss.NewStyle().Foreground(ParseHex("#a6b3cc"))

	return t
}
