package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// minBoxWidth is the narrowest a drawn box may get before we stop shrinking it.
// Without this floor, `width - N` goes negative on tiny terminals and
// strings.Repeat panics with "negative Repeat count".
const minBoxWidth = 20

// Clip trims rendered content to a viewport `height` rows tall starting at
// scrollY, so long views stay reachable on short terminals instead of
// overflowing off-screen. It returns the visible slice and the largest valid
// scroll offset, so callers can clamp their own scroll state.
//
// A "▲ N more" / "▼ N more" indicator replaces the first/last visible row
// whenever content extends past the viewport in that direction.
func Clip(r *lipgloss.Renderer, content string, height, scrollY int, theme Theme) (string, int) {
	lines := strings.Split(content, "\n")
	// Trailing newline produces a final empty element that isn't a real row.
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}

	if height < 1 {
		height = 1
	}
	maxScroll := len(lines) - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scrollY > maxScroll {
		scrollY = maxScroll
	}
	if scrollY < 0 {
		scrollY = 0
	}

	if len(lines) <= height {
		return strings.Join(lines, "\n"), 0
	}

	visible := make([]string, height)
	copy(visible, lines[scrollY:scrollY+height])

	hintStyle := r.NewStyle().Foreground(lipgloss.Color(theme.Accent))
	if scrollY > 0 {
		visible[0] = hintStyle.Render(padLeft("▲ " + itoa(scrollY) + " more above"))
	}
	if below := maxScroll - scrollY; below > 0 {
		visible[height-1] = hintStyle.Render(padLeft("▼ " + itoa(below) + " more below  [↓/j · PgDn · G]"))
	}
	return strings.Join(visible, "\n"), maxScroll
}

// MaxScroll reports how far content can scroll inside a viewport of `height`
// rows. Callers use it to clamp scroll state without rendering twice.
func MaxScroll(content string, height int) int {
	lines := strings.Split(content, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	if height < 1 {
		height = 1
	}
	if n := len(lines) - height; n > 0 {
		return n
	}
	return 0
}

// fitToWidth forces a styled string to exactly targetWidth display columns,
// truncating when it overflows and padding when it falls short. Padding alone
// isn't enough: an over-long value pushes a box's right border out of line.
func fitToWidth(r *lipgloss.Renderer, content string, targetWidth int) string {
	if targetWidth < 0 {
		targetWidth = 0
	}
	if lipgloss.Width(content) > targetWidth {
		content = r.NewStyle().MaxWidth(targetWidth).Render(content)
	}
	return padToWidth(content, targetWidth)
}

func padLeft(s string) string { return "  " + s }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// wrapWidth wraps text to maxWidth *display columns*, preserving any ANSI
// escape sequences the words carry. The previous implementation compared
// byte length against maxWidth, so a styled 43-column title measuring 92
// bytes wrapped after ~2 visible words over a real SSH connection.
func wrapWidth(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return text
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}
	var lines []string
	var cur strings.Builder
	curW := 0
	for _, word := range words {
		w := lipgloss.Width(word)
		if curW > 0 && curW+1+w > maxWidth {
			lines = append(lines, cur.String())
			cur.Reset()
			curW = 0
		}
		if curW > 0 {
			cur.WriteString(" ")
			curW++
		}
		cur.WriteString(word)
		curW += w
	}
	if curW > 0 {
		lines = append(lines, cur.String())
	}
	return strings.Join(lines, "\n")
}

// StripAnsiForTest exposes stripAnsi to tests in other packages.
func StripAnsiForTest(s string) string { return stripAnsi(s) }

// Small numeric helpers used across the view layer.

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
