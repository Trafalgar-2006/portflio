package views

import "github.com/charmbracelet/lipgloss"

// Theme holds all color tokens for the portfolio.
// All render functions receive a Theme so every screen reacts to theme switches.
type Theme struct {
	Name string

	// Core palette
	Primary   lipgloss.Color // main accent
	Secondary lipgloss.Color // second accent
	Accent    lipgloss.Color // gold/highlight
	Success   lipgloss.Color // positive / live indicator
	Warning   lipgloss.Color // orange / WIP indicator
	Purple    lipgloss.Color // framework / tag color

	// Text levels
	Text    lipgloss.Color // bright readable text
	DimMid  lipgloss.Color // medium dim
	Dim     lipgloss.Color // darkest readable dim
	VeryDim lipgloss.Color // muted metadata + keyboard hints (must stay legible)

	// UI chrome
	BoxBorder  lipgloss.Color // box drawing lines
	FooterBg   lipgloss.Color // footer bar background
	FooterText lipgloss.Color // footer bar muted text
	TabActive  lipgloss.Color // active tab highlight bg

	// Matrix rain colors
	MatrixHead   lipgloss.Color // column head (brightest)
	MatrixBright lipgloss.Color // bright trail
	MatrixMid    lipgloss.Color // mid trail
	MatrixDim    lipgloss.Color // dim trail
	MatrixLocked lipgloss.Color // crystallised name color

	// Star twinkle
	StarBright lipgloss.Color
	StarDim    lipgloss.Color

	// Scanline / glitch overlay
	ScanlineColor lipgloss.Color
}

// ──────────────────────────────────────────────────────────────
// Theme registry — [t] cycles through these in order
// ──────────────────────────────────────────────────────────────

var Themes = []Theme{
	ThemeDracula,
	ThemeMatrix,
	ThemeAmber,
	ThemeNord,
	ThemeCyberpunk,
}

// ──────────────────────────────────────────────────────────────
// DRACULA — rich purple-dark with vivid cyan + hot pink
// The classic. Deep contrast, saturated accents.
// ──────────────────────────────────────────────────────────────
var ThemeDracula = Theme{
	Name:      "dracula",
	Primary:   "#8BE9FD", // dracula cyan — bright and clean
	Secondary: "#FF79C6", // dracula pink — vivid hot pink
	Accent:    "#F1FA8C", // dracula yellow — warm highlight
	Success:   "#50FA7B", // dracula green — electric
	Warning:   "#FFB86C", // dracula orange
	Purple:    "#BD93F9", // dracula purple

	Text:    "#F8F8F2", // dracula foreground — near-white
	DimMid:  "#6272A4", // dracula comment blue — readable dim
	Dim:     "#44475A", // dracula selection — dark dim
	VeryDim: "#565B7A", // muted comment blue — legible on dark

	BoxBorder:  "#6272A4",
	FooterBg:   "#21222C",
	FooterText: "#7D82A8",
	TabActive:  "#8BE9FD",

	MatrixHead:   "#FFFFFF",
	MatrixBright: "#50FA7B",
	MatrixMid:    "#00CC55",
	MatrixDim:    "#004422",
	MatrixLocked: "#8BE9FD",

	StarBright:    "#F1FA8C",
	StarDim:       "#44475A",
	ScanlineColor: "#191A21",
}

// ──────────────────────────────────────────────────────────────
// MATRIX — pure green-on-black. No colour. No compromise.
// ──────────────────────────────────────────────────────────────
var ThemeMatrix = Theme{
	Name:      "matrix",
	Primary:   "#00FF41", // the iconic matrix green
	Secondary: "#00DD33", // slightly dimmer green
	Accent:    "#AAFFAA", // near-white green for highlights
	Success:   "#00FF41",
	Warning:   "#99FF33", // yellow-green for WIP
	Purple:    "#00BB22", // darker green for variety

	Text:    "#CCFFCC", // light green text
	DimMid:  "#008822",
	Dim:     "#004411",
	VeryDim: "#00702E",

	BoxBorder:  "#006622",
	FooterBg:   "#000C00",
	FooterText: "#00B04A",
	TabActive:  "#00FF41",

	MatrixHead:   "#FFFFFF", // pure white column head
	MatrixBright: "#00FF41",
	MatrixMid:    "#00AA00",
	MatrixDim:    "#003300",
	MatrixLocked: "#FFFFFF", // name crystallises in white

	StarBright:    "#00FF41",
	StarDim:       "#003311",
	ScanlineColor: "#000E00",
}

// ──────────────────────────────────────────────────────────────
// AMBER — phosphor CRT. 1970s terminal energy.
// Everything amber. Nothing else exists.
// ──────────────────────────────────────────────────────────────
var ThemeAmber = Theme{
	Name:      "amber",
	Primary:   "#FFAA00", // classic amber phosphor
	Secondary: "#FF7700", // deeper orange
	Accent:    "#FFE066", // bright highlight amber
	Success:   "#CCFF00", // yellow-green for Live status
	Warning:   "#FF5500", // burnt orange for WIP
	Purple:    "#FF9944", // orange-gold for tags

	Text:    "#FFD080", // warm amber text
	DimMid:  "#AA6600",
	Dim:     "#664400",
	VeryDim: "#8A6A20",

	BoxBorder:  "#885500",
	FooterBg:   "#0A0600",
	FooterText: "#C08830",
	TabActive:  "#FFAA00",

	MatrixHead:   "#FFEEAA", // warm white head
	MatrixBright: "#FFAA00",
	MatrixMid:    "#AA6600",
	MatrixDim:    "#441E00",
	MatrixLocked: "#FFE066",

	StarBright:    "#FFE066",
	StarDim:       "#553300",
	ScanlineColor: "#140A00",
}

// ──────────────────────────────────────────────────────────────
// NORD — true Arctic palette. Icy blues, sage greens.
// Based on the official Nord color specification.
// ──────────────────────────────────────────────────────────────
var ThemeNord = Theme{
	Name:      "nord",
	Primary:   "#88C0D0", // Nord8 — frost blue (the signature color)
	Secondary: "#B48EAD", // Nord15 — muted purple
	Accent:    "#EBCB8B", // Nord13 — aurora yellow
	Success:   "#A3BE8C", // Nord14 — aurora green
	Warning:   "#D08770", // Nord12 — aurora orange
	Purple:    "#81A1C1", // Nord9 — frost blue-grey

	Text:    "#ECEFF4", // Nord6 — snow storm white
	DimMid:  "#7A8899",
	Dim:     "#4C566A", // Nord3 — polar night light
	VeryDim: "#6E7A8F", // Nord3-ish — legible muted slate

	BoxBorder:  "#5E81AC", // Nord10 — frost blue dark
	FooterBg:   "#242933",
	FooterText: "#8B9BB4",
	TabActive:  "#88C0D0",

	MatrixHead:   "#ECEFF4",
	MatrixBright: "#88C0D0",
	MatrixMid:    "#5E81AC",
	MatrixDim:    "#2E3440",
	MatrixLocked: "#EBCB8B",

	StarBright:    "#EBCB8B",
	StarDim:       "#3B4252",
	ScanlineColor: "#1E2430",
}

// ──────────────────────────────────────────────────────────────
// CYBERPUNK — neon pink + electric blue on deep purple-black.
// Press [t] until you reach this. You've earned it.
// ──────────────────────────────────────────────────────────────
var ThemeCyberpunk = Theme{
	Name:      "cyberpunk",
	Primary:   "#FF007F", // electric hot pink
	Secondary: "#00FFFF", // electric cyan
	Accent:    "#FFE600", // neon yellow
	Success:   "#00FF88", // neon green
	Warning:   "#FF6600", // neon orange
	Purple:    "#CC00FF", // neon purple

	Text:    "#FFFFFF",
	DimMid:  "#AA44AA",
	Dim:     "#550055",
	VeryDim: "#8A4477",

	BoxBorder:  "#880055",
	FooterBg:   "#0A000A",
	FooterText: "#BE5C9C",
	TabActive:  "#FF007F",

	MatrixHead:   "#FFFFFF",
	MatrixBright: "#FF007F",
	MatrixMid:    "#AA0055",
	MatrixDim:    "#330011",
	MatrixLocked: "#00FFFF",

	StarBright:    "#FFE600",
	StarDim:       "#440033",
	ScanlineColor: "#0E000A",
}
