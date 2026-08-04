package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ClientInfo is what the greeting card knows about the visitor's terminal.
// The model fills this in from the SSH session.
type ClientInfo struct {
	User      string
	Client    string // "OpenSSH_9.6"
	Term      string // "xterm-256color"
	Width     int
	Height    int
	KeyType   string // "ssh-ed25519", or "" when they used keyboard-interactive
	MaskedIP  string
	SessionID string
	Connected string // "00:42"
	Colors    int    // detected colour depth
}

// neofetchLogo is the small block-letter mark shown beside the spec table.
var neofetchLogo = []string{
	"      ▄▄▄▄▄▄▄      ",
	"    ▄█████████▄    ",
	"   ███▀     ▀███   ",
	"  ███   ▄▄▄   ███  ",
	"  ██   █████   ██  ",
	"  ███   ▀▀▀   ███  ",
	"   ███▄     ▄███   ",
	"    ▀█████████▀    ",
	"      ▀▀▀▀▀▀▀      ",
}

// RenderNeofetch draws a system-spec greeting card describing the visitor's
// own connection — the terminal equivalent of "you are here".
func RenderNeofetch(r *lipgloss.Renderer, width, height int, info ClientInfo, theme Theme) string {
	logoS := r.NewStyle().Foreground(lipgloss.Color(theme.Primary)).Bold(true)
	keyS := r.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Bold(true)
	valS := r.NewStyle().Foreground(lipgloss.Color(theme.Text))
	dimS := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))
	hintS := r.NewStyle().Foreground(lipgloss.Color(theme.VeryDim)).Italic(true)
	ruleS := r.NewStyle().Foreground(lipgloss.Color(theme.BoxBorder))

	user := info.User
	if user == "" {
		user = "guest"
	}
	title := user + "@mohith.is-a.dev"

	keyAuth := info.KeyType
	if keyAuth == "" {
		keyAuth = "keyboard-interactive (no key offered)"
	}
	colors := "unknown"
	if info.Colors > 0 {
		colors = fmt.Sprintf("%d", info.Colors)
	}

	rows := [][2]string{
		{"session", info.SessionID},
		{"uptime", info.Connected},
		{"client", info.Client},
		{"auth", keyAuth},
		{"term", info.Term},
		{"size", fmt.Sprintf("%d×%d cells", info.Width, info.Height)},
		{"colors", colors},
		{"origin", info.MaskedIP},
		{"host", "ssh.mohith.is-a.dev:41074"},
		{"served by", "Go · Bubbletea · Wish"},
		{"theme", theme.Name},
	}

	// Widest key, so the values line up in a column.
	keyW := 0
	for _, kv := range rows {
		if len(kv[0]) > keyW {
			keyW = len(kv[0])
		}
	}

	var right []string
	right = append(right, keyS.Render(title))
	right = append(right, ruleS.Render(strings.Repeat("─", lipgloss.Width(title))))
	for _, kv := range rows {
		right = append(right,
			keyS.Render(fmt.Sprintf("%-*s", keyW, kv[0]))+dimS.Render(" : ")+valS.Render(kv[1]))
	}
	right = append(right, "")
	// Colour swatches, the way neofetch signs off.
	var swatch strings.Builder
	for _, c := range []lipgloss.Color{
		lipgloss.Color(theme.Primary), lipgloss.Color(theme.Secondary),
		lipgloss.Color(theme.Accent), lipgloss.Color(theme.Success),
		lipgloss.Color(theme.Warning), lipgloss.Color(theme.Purple),
		lipgloss.Color(theme.Text), lipgloss.Color(theme.DimMid),
	} {
		swatch.WriteString(r.NewStyle().Foreground(c).Render("███"))
	}
	right = append(right, swatch.String())

	// Lay the logo beside the table when there's room, above it when there isn't.
	logoW := 0
	for _, l := range neofetchLogo {
		if w := lipgloss.Width(l); w > logoW {
			logoW = w
		}
	}
	sideBySide := width >= logoW+46

	var b strings.Builder
	b.WriteString("\n")
	if sideBySide {
		n := len(neofetchLogo)
		if len(right) > n {
			n = len(right)
		}
		for i := 0; i < n; i++ {
			left := ""
			if i < len(neofetchLogo) {
				left = logoS.Render(neofetchLogo[i])
			}
			row := ""
			if i < len(right) {
				row = right[i]
			}
			b.WriteString("  " + padToWidth(left, logoW) + "   " + row + "\n")
		}
	} else {
		for _, l := range neofetchLogo {
			b.WriteString("  " + logoS.Render(l) + "\n")
		}
		b.WriteString("\n")
		for _, row := range right {
			b.WriteString("  " + row + "\n")
		}
	}

	b.WriteString("\n  " + hintS.Render("[any key to continue]") + "\n")
	return b.String()
}
