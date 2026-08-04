package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// AdminStats is the live server picture shown to an authenticated operator.
type AdminStats struct {
	Uptime        time.Duration
	Sessions      int64
	PeakSessions  int64
	TotalVisits   int64
	Guestbook     int
	Subscribers   int
	GoVersion     string
	Commit        string
	Built         string
	NumGoroutine  int
	HeapMB        float64
	SysMB         float64
	GCRuns        uint32
	Projects      int
	SyncedRepos   int
	LastSync      time.Time
	LastSyncErr   string
	RateRemaining string
	WakaToday     string
	WakaWeek      string
	WakaTop       []LangStat
}

// LangStat is one language row in the coding-time breakdown.
type LangStat struct {
	Name    string
	Percent float64
	Text    string
}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Second)
	days := int(d.Hours()) / 24
	h := int(d.Hours()) % 24
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %02dh %02dm", days, h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dh %02dm %02ds", h, m, s)
	}
	return fmt.Sprintf("%dm %02ds", m, s)
}

// RenderAdmin draws the operator dashboard.
func RenderAdmin(r *lipgloss.Renderer, width, height int, st AdminStats, theme Theme) string {
	cyanS := r.NewStyle().Foreground(lipgloss.Color(theme.Primary))
	goldS := r.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Bold(true)
	dimS := r.NewStyle().Foreground(lipgloss.Color(theme.Dim))
	midS := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))
	textS := r.NewStyle().Foreground(lipgloss.Color(theme.Text))
	okS := r.NewStyle().Foreground(lipgloss.Color(theme.Success))
	warnS := r.NewStyle().Foreground(lipgloss.Color(theme.Warning))
	boxS := r.NewStyle().Foreground(lipgloss.Color(theme.BoxBorder))
	hintS := r.NewStyle().Foreground(lipgloss.Color(theme.VeryDim)).Italic(true)

	boxW := 60
	if width-6 < boxW {
		boxW = width - 6
	}
	if boxW < minBoxWidth {
		boxW = minBoxWidth
	}

	section := func(b *strings.Builder, title string, rows [][2]string) {
		b.WriteString("  " + goldS.Render("◆ "+title) + "\n")
		b.WriteString("  " + boxS.Render("╭"+strings.Repeat("─", boxW-2)+"╮") + "\n")
		for _, kv := range rows {
			line := " " + midS.Render(fmt.Sprintf("%-16s", kv[0])) + textS.Render(kv[1])
			b.WriteString("  " + boxS.Render("│") + fitToWidth(r, line, boxW-2) + boxS.Render("│") + "\n")
		}
		b.WriteString("  " + boxS.Render("╰"+strings.Repeat("─", boxW-2)+"╯") + "\n\n")
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(" " + cyanS.Bold(true).Render("✦ Admin — live server stats") + "  " +
		okS.Render("● authenticated") + "\n")
	b.WriteString(dimS.Render("  "+strings.Repeat("─", boxW)) + "\n\n")

	section(&b, "Service", [][2]string{
		{"uptime", fmtDuration(st.Uptime)},
		{"commit", st.Commit},
		{"built", st.Built},
		{"go", st.GoVersion},
	})

	section(&b, "Traffic", [][2]string{
		{"live sessions", fmt.Sprintf("%d", st.Sessions)},
		{"peak concurrent", fmt.Sprintf("%d", st.PeakSessions)},
		{"total visits", fmt.Sprintf("%d", st.TotalVisits)},
		{"guestbook", fmt.Sprintf("%d messages", st.Guestbook)},
		{"gb listeners", fmt.Sprintf("%d", st.Subscribers)},
	})

	section(&b, "Runtime", [][2]string{
		{"goroutines", fmt.Sprintf("%d", st.NumGoroutine)},
		{"heap", fmt.Sprintf("%.1f MB", st.HeapMB)},
		{"sys", fmt.Sprintf("%.1f MB", st.SysMB)},
		{"gc runs", fmt.Sprintf("%d", st.GCRuns)},
	})

	syncRows := [][2]string{
		{"projects", fmt.Sprintf("%d", st.Projects)},
		{"synced repos", fmt.Sprintf("%d", st.SyncedRepos)},
	}
	if st.LastSync.IsZero() {
		syncRows = append(syncRows, [2]string{"last sync", "not run yet"})
	} else {
		syncRows = append(syncRows, [2]string{"last sync", relativeTime(st.LastSync)})
	}
	syncRows = append(syncRows, [2]string{"api budget", st.RateRemaining})
	section(&b, "GitHub sync", syncRows)
	if st.LastSyncErr != "" {
		b.WriteString("  " + warnS.Render("! last sync error: "+st.LastSyncErr) + "\n\n")
	}

	// WakaTime is optional — omit the panel entirely when unconfigured.
	if st.WakaToday != "" || len(st.WakaTop) > 0 {
		b.WriteString("  " + goldS.Render("◆ Coding activity (WakaTime)") + "\n")
		b.WriteString("  " + boxS.Render("╭"+strings.Repeat("─", boxW-2)+"╮") + "\n")
		for _, kv := range [][2]string{{"today", st.WakaToday}, {"this week", st.WakaWeek}} {
			line := " " + midS.Render(fmt.Sprintf("%-16s", kv[0])) + textS.Render(kv[1])
			b.WriteString("  " + boxS.Render("│") + fitToWidth(r, line, boxW-2) + boxS.Render("│") + "\n")
		}
		for _, l := range st.WakaTop {
			bar := renderPercentBar(r, l.Percent, 20, theme)
			line := " " + midS.Render(fmt.Sprintf("%-12s", trimTo(l.Name, 12))) + bar + " " + dimS.Render(l.Text)
			b.WriteString("  " + boxS.Render("│") + fitToWidth(r, line, boxW-2) + boxS.Render("│") + "\n")
		}
		b.WriteString("  " + boxS.Render("╰"+strings.Repeat("─", boxW-2)+"╯") + "\n\n")
	}

	b.WriteString("  " + hintS.Render("[↑↓ scroll · esc back]") + "\n")
	return b.String()
}

func trimTo(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// renderPercentBar draws a proportional bar for a 0..100 value.
func renderPercentBar(r *lipgloss.Renderer, pct float64, width int, theme Theme) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	return r.NewStyle().Foreground(lipgloss.Color(theme.Primary)).Render(strings.Repeat("█", filled)) +
		r.NewStyle().Foreground(lipgloss.Color(theme.Dim)).Render(strings.Repeat("░", width-filled))
}

// RenderAdminDenied is what a non-operator sees if they try the admin route.
func RenderAdminDenied(r *lipgloss.Renderer, theme Theme) string {
	warnS := r.NewStyle().Foreground(lipgloss.Color(theme.Warning)).Bold(true)
	dimS := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))
	return "\n\n  " + warnS.Render("⚠ admin view requires an authorised SSH key") + "\n\n" +
		"  " + dimS.Render("Connect with the operator key to see live server stats.") + "\n\n" +
		"  " + dimS.Render("[esc] back") + "\n"
}
