package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/trafalgar-2006/ssh-portfolio/config"
)

type Project struct {
	Title       string
	Description string
	Tags        []string
	Highlight   string
	Status      string // "Live", "Research", "WIP"
	GitHubURL   string
	Pinned      bool // never replaced by the GitHub auto-sync worker
	FromGitHub  bool // added by the sync worker rather than content.yaml
}

var AllProjects = []Project{
	{
		Title:       "EmbedGen — Embedded Systems LLM",
		Description: "Custom LLM fine-tuned on embedded-systems codebases using LoRA adapters. Quantized to GGUF for deployment on edge devices. Trained on RTX A6000 at Manipal research lab.",
		Tags:        []string{"Python", "PyTorch", "LoRA", "GGUF", "CUDA"},
		Status:      "WIP",
		GitHubURL:   "github.com/trafalgar-2006/EmbedGen",
	},
	{
		Title:       "ISRO LEOS — Sim-to-Real Vision",
		Description: "Synthetic data generation using BlenderProc for 6,000+ satellite images with COCO annotations. YOLOv7+Segmentation on NVIDIA Jetson via TensorRT. GAN-based style transfer with Unreal Engine 5.",
		Tags:        []string{"BlenderProc", "YOLOv7", "TensorRT", "Python", "OpenCV"},
		Highlight:   "22 FPS real-time",
		Status:      "Research",
		GitHubURL:   "",
	},
	{
		Title:       "Autonomous Trading System",
		Description: "AI-driven swing trader with market-wide signal generation, paper trading via Alpaca API, risk management, and automated order execution. Deployed on Oracle Cloud Always Free tier.",
		Tags:        []string{"Python", "SQLite", "Alpaca API", "Oracle Cloud"},
		Status:      "Live",
		GitHubURL:   "github.com/trafalgar-2006/autonomous-trading-agent",
	},
	{
		Title:       "Webcraft Studios Platform",
		Description: "Full digital agency platform with SEO-driven architecture, scalable AI pipelines, and performance-optimized delivery for global client base.",
		Tags:        []string{"React", "Node.js", "MongoDB", "AWS", "Python"},
		Status:      "Live",
		GitHubURL:   "",
	},
	{
		Title:       "Bird Sound Species Detector",
		Description: "CNN-based audio classifier for bird species identification using spectrogram analysis. Trained on a custom dataset.",
		Tags:        []string{"Python", "TensorFlow", "CNNs"},
		Highlight:   "90%+ accuracy",
		Status:      "Research",
		GitHubURL:   "",
	},
	{
		Title:       "Frog Call Classifier",
		Description: "ML pipeline for frog species identification across 803+ species using MFCC-based feature extraction from environmental audio recordings.",
		Tags:        []string{"Python", "Librosa", "Scikit-learn"},
		Status:      "Research",
		GitHubURL:   "",
	},
	{
		Title:       "SSH Portfolio (this one)",
		Description: "This portfolio — an interactive TUI accessible over SSH from anywhere in the world. Auto-deploys from GitHub via Railway.",
		Tags:        []string{"Go", "Docker", "Railway"},
		Status:      "Live",
		GitHubURL:   "github.com/trafalgar-2006/portfolio",
	},
}

// LoadFromConfig overwrites AllProjects and AllContacts from the loaded YAML config.
// Called at startup if content.yaml was found. Falls back to hardcoded data if not.
func LoadFromConfig() {
	if config.Loaded == nil {
		return
	}
	var projects []Project
	for _, p := range config.Loaded.Projects {
		projects = append(projects, Project{
			Title:       p.Title,
			Description: p.Description,
			Tags:        p.Tags,
			Status:      p.Status,
			GitHubURL:   p.GitHubURL,
			Highlight:   p.Highlight,
			Pinned:      p.Pinned,
		})
	}
	if len(projects) > 0 {
		SetProjects(projects)
	}
	loadContactsFromConfig()
	loadNarrativeFromConfig()
}


func tagColor(tag string, theme Theme) lipgloss.Color {
	langs := map[string]bool{
		"Python": true, "Go": true, "JavaScript": true, "TypeScript": true,
		"C++": true, "Rust": true, "SQL": true, "HTML": true,
	}
	ml := map[string]bool{
		"PyTorch": true, "TensorFlow": true, "LoRA": true, "GGUF": true, "ONNX": true,
		"YOLOv7": true, "TensorRT": true, "OpenCV": true, "CNNs": true, "GANs": true,
		"Scikit-learn": true, "Librosa": true, "BlenderProc": true, "CUDA": true,
		"ResNet": true, "Transformers": true, "NumPy": true, "Pandas": true,
	}
	cloud := map[string]bool{
		"AWS": true, "Firebase": true, "Docker": true, "Railway": true,
		"Vercel": true, "Oracle Cloud": true, "Fly.io": true, "Alpaca API": true,
		"Wish": true, "SSH": true,
	}
	db := map[string]bool{"MongoDB": true, "PostgreSQL": true, "SQLite": true, "Redis": true}

	if langs[tag] { return theme.Primary }
	if ml[tag]    { return theme.Secondary }
	if cloud[tag] { return theme.Success }
	if db[tag]    { return theme.Warning }
	return theme.Purple
}

func statusDot(status string, theme Theme) string {
	switch status {
	case "Live":     return string(theme.Success)
	case "WIP":      return string(theme.Warning)
	case "Research": return string(theme.Purple)
	default:         return string(theme.Dim)
	}
}

func statusBadge(r *lipgloss.Renderer, status string, theme Theme) string {
	switch status {
	case "Live":
		return r.NewStyle().Foreground(theme.Success).Bold(true).Render("[Live]")
	case "WIP":
		return r.NewStyle().Foreground(theme.Warning).Bold(true).Render("[WIP]")
	case "Research":
		return r.NewStyle().Foreground(theme.Purple).Bold(true).Render("[Research]")
	default:
		return ""
	}
}

// sparklineStr returns a 6-char animated sparkline for a project row.
func sparklineStr(proj Project, tickCount int) string {
	const bars = "▁▂▃▄▅▆▇"
	runes := []rune(bars)
	var pattern []int
	switch proj.Status {
	case "Live":
		pattern = []int{2, 4, 6, 5, 3, 4, 6, 5} // animated wave
	case "WIP":
		pattern = []int{1, 2, 1, 3, 2, 1, 2, 3}
	case "Research":
		pattern = []int{1, 2, 3, 2, 1, 2, 1, 2}
	default:
		pattern = []int{0, 0, 1, 0, 0, 1, 0, 0}
	}
	offset := 0
	if proj.Status == "Live" {
		offset = (tickCount / 3) % len(pattern)
	}
	var sb strings.Builder
	for i := 0; i < 5; i++ {
		sb.WriteRune(runes[pattern[(offset+i)%len(pattern)]])
	}
	return sb.String()
}

// ProjectListTop returns the index of the first visible project row, keeping
// the cursor inside a window of `rows` entries. The model calls this too, so
// its stored scroll offset and what's drawn can't disagree.
func ProjectListTop(cursor, scroll, rows int) int {
	if rows < 1 {
		rows = 1
	}
	total := ProjectCount()
	top := scroll
	if top > cursor {
		top = cursor // cursor moved above the window — follow it up
	}
	if cursor >= top+rows {
		top = cursor - rows + 1 // ...and below it — follow it down
	}
	if max := total - rows; top > max {
		top = max
	}
	if top < 0 {
		top = 0
	}
	return top
}

// ProjectListRows is how many project rows fit in the list panel at this height.
func ProjectListRows(height int) int {
	rows := height - 8
	if rows < 3 {
		rows = 3
	}
	if total := ProjectCount(); rows > total {
		rows = total
	}
	return rows
}

func RenderProjects(r *lipgloss.Renderer, width, height, cursor, scroll, projectsReveal, tagPopReveal int, livePulse bool, highlightY float64, decryptIdx int, decryptRunes []rune, tickCount, ghostCursor1, ghostFade1, ghostCursor2, ghostFade2 int, theme Theme) string {
	goldStyle       := r.NewStyle().Foreground(theme.Accent).Bold(true)
	cyanStyle       := r.NewStyle().Foreground(theme.Primary)
	dimStyle        := r.NewStyle().Foreground(theme.Dim)
	dimMidStyle     := r.NewStyle().Foreground(theme.DimMid)
	orangeStyle     := r.NewStyle().Foreground(theme.Warning)
	greenStyle      := r.NewStyle().Foreground(theme.Success).Bold(true)
	selectedBg      := r.NewStyle().Background(theme.BoxBorder).Foreground(theme.Primary).Bold(true)
	unselectedStyle := r.NewStyle().Foreground(theme.DimMid)
	boxStyle        := r.NewStyle().Foreground(theme.BoxBorder)
	decryptStyle    := r.NewStyle().Foreground(theme.Text)
	hintStyle       := r.NewStyle().Foreground(theme.VeryDim).Italic(true)

	// One snapshot per frame: the GitHub sync worker can replace the list
	// mid-render otherwise.
	projects := Projects()

	// Nothing to show — bail before any indexing. Reachable when content.yaml
	// parses to zero projects.
	if len(projects) == 0 {
		return "\n  " + dimStyle.Render("No projects configured.") + "\n"
	}

	// Visual cursor row from lerp (rounded)
	visualCursor := int(highlightY + 0.5)
	if visualCursor < 0                  { visualCursor = 0 }
	if visualCursor >= len(projects)  { visualCursor = len(projects) - 1 }

	if cursor < 0                 { cursor = 0 }
	if cursor >= len(projects) { cursor = len(projects) - 1 }

	// ── Layout math ───────────────────────────────────────────────
	totalW  := width - 4
	leftW   := 32
	dividerW := 1
	rightW  := totalW - leftW - dividerW - 2
	if rightW < 30 { rightW = 30 }

	p := projects[cursor]

	// The list gets its own window so it can't outgrow the terminal: header
	// (3 rows) + footer (2 rows) + the surrounding chrome.
	listRows := ProjectListRows(height)
	first := ProjectListTop(cursor, scroll, listRows)
	last := first + listRows

	// ── LEFT PANEL — project list ──────────────────────────────────
	var left strings.Builder
	left.WriteString(cyanStyle.Bold(true).Render(" ✦ Projects") + "\n")
	counter := fmt.Sprintf("%d projects", len(projects))
	if listRows < len(projects) {
		counter = fmt.Sprintf("%d/%d projects", cursor+1, len(projects))
	}
	left.WriteString(dimStyle.Render(" "+counter) + "\n")
	if first > 0 {
		left.WriteString(boxStyle.Render(" "+strings.Repeat("─", leftW-6)) + dimStyle.Render(" ▲") + "\n")
	} else {
		left.WriteString(boxStyle.Render(" "+strings.Repeat("─", leftW-2)) + "\n")
	}

	for i, proj := range projects {
		if i < first || i >= last {
			continue
		}
		if i >= projectsReveal {
			if i == projectsReveal {
				scanChar := []string{"▶", "▷", " "}[(projectsReveal/2)%3]
				left.WriteString(dimStyle.Render(" "+scanChar) + "\n")
			} else {
				left.WriteString("\n")
			}
			continue
		}

		isSelected := i == cursor
		isHover    := i == visualCursor && visualCursor != cursor
		isGhost1   := i == ghostCursor1 && ghostFade1 > 0 && !isSelected
		isGhost2   := i == ghostCursor2 && ghostFade2 > 0 && !isSelected && !isGhost1
		num        := fmt.Sprintf("%d", i+1)

		// Budget the row to exactly leftW columns:
		//   " " + num + " " + title + pad + " " + spark(5) + dot(1)
		// Anything wider pushes the divider column out of alignment.
		const trailW = 7 // space + 5-cell sparkline + status dot
		maxTitleW := leftW - trailW - len(num) - 2
		if maxTitleW < 1 { maxTitleW = 1 }

		title := proj.Title
		if len([]rune(title)) > maxTitleW {
			runes := []rune(title)
			title = string(runes[:maxTitleW-1]) + "…"
		}
		titlePad := maxTitleW - len([]rune(title))
		if titlePad < 0 { titlePad = 0 }

		sparkColor := lipgloss.Color(theme.Success)
		if proj.Status == "WIP"      { sparkColor = theme.Warning }
		if proj.Status == "Research" { sparkColor = theme.Purple }
		sparkS := r.NewStyle().Foreground(sparkColor)
		spark  := sparkS.Render(sparklineStr(proj, tickCount))

		pad := strings.Repeat(" ", titlePad)
		var line string
		if isSelected {
			// Selected row carries a background, so the padding must be inside
			// the styled span for the highlight bar to span the full column.
			line = selectedBg.Render(" "+num+" "+title+pad)
		} else if isGhost1 {
			line = r.NewStyle().Foreground(lipgloss.Color(theme.DimMid)).Render(" "+num+" "+title) + pad
		} else if isGhost2 {
			line = r.NewStyle().Foreground(lipgloss.Color(theme.Dim)).Render(" "+num+" "+title) + pad
		} else if isHover {
			line = r.NewStyle().Foreground(theme.Primary).Render(" "+num+" ") + dimMidStyle.Render(title) + pad
		} else {
			line = unselectedStyle.Render(" "+num+" ") + dimStyle.Render(title) + pad
		}

		// Status dot
		var dot string
		switch proj.Status {
		case "Live":
			if livePulse { dot = r.NewStyle().Foreground(theme.Success).Render("●") } else { dot = dimStyle.Render("○") }
		case "WIP":
			dot = r.NewStyle().Foreground(theme.Warning).Render("◐")
		case "Research":
			dot = r.NewStyle().Foreground(theme.Purple).Render("◇")
		default:
			dot = " "
		}
		left.WriteString(fitToWidth(r, line+" "+spark+dot, leftW) + "\n")
	}

	if last < len(projects) {
		left.WriteString(boxStyle.Render(" "+strings.Repeat("─", leftW-6)) + dimStyle.Render(" ▼") + "\n")
	} else {
		left.WriteString(boxStyle.Render(" "+strings.Repeat("─", leftW-2)) + "\n")
	}
	left.WriteString(hintStyle.Render(" jk move · PgDn · gg/G · t") + "\n")

	// ── RIGHT PANEL — selected project detail with box border ──────
	var rightLines []string

	// Box top/bottom, plus a row helper that closes the box on the right.
	// Inner width is the column count between the two │ borders.
	inner     := rightW - 2
	boxTop    := boxStyle.Render("╭"+strings.Repeat("─", inner)+"╮")
	boxBottom := boxStyle.Render("╰"+strings.Repeat("─", inner)+"╯")
	borderL   := boxStyle.Render("│")
	borderR   := boxStyle.Render("│")
	row := func(content string) string {
		return borderL + fitToWidth(r, " "+content, inner) + borderR
	}
	rule := func() string { return row(boxStyle.Render(strings.Repeat("─", inner-2))) }

	// Title + status badge
	titleLine := goldStyle.Render(p.Title)
	if p.Status == "Live" {
		if livePulse {
			titleLine += "  " + greenStyle.Render("● Live")
		} else {
			titleLine += "  " + dimStyle.Render("○ Live")
		}
	} else {
		titleLine += "  " + statusBadge(r, p.Status, theme)
	}
	if p.Highlight != "" {
		titleLine += "  " + greenStyle.Render("⚡ "+p.Highlight)
	}

	rightLines = append(rightLines, boxTop)
	for _, line := range strings.Split(wrapWidth(titleLine, inner-2), "\n") {
		rightLines = append(rightLines, row(line))
	}
	rightLines = append(rightLines, rule())
	rightLines = append(rightLines, row(""))

	// Description — decrypt reveal
	var descText string
	if len(decryptRunes) > 0 && decryptIdx < len(decryptRunes) {
		desc   := []rune(p.Description)
		merged := make([]rune, len(desc))
		for j := range desc {
			if j < decryptIdx {
				merged[j] = desc[j]
			} else if j < len(decryptRunes) {
				merged[j] = decryptRunes[j]
			} else {
				merged[j] = desc[j]
			}
		}
		descText = wrapWidth(string(merged), inner-2)
	} else {
		descText = wrapWidth(p.Description, inner-2)
	}
	for _, line := range strings.Split(descText, "\n") {
		rightLines = append(rightLines, row(decryptStyle.Render(line)))
	}
	rightLines = append(rightLines, row(""))

	// GitHub link
	rightLines = append(rightLines, rule())
	if p.GitHubURL != "" {
		rightLines = append(rightLines, row(orangeStyle.Render("→ "+p.GitHubURL)))
	} else {
		rightLines = append(rightLines, row(dimStyle.Render("→ Private / Institutional repo")))
	}

	// Tags — pop in
	showCount := tagPopReveal
	if showCount > len(p.Tags) { showCount = len(p.Tags) }
	var tagParts []string
	for _, t := range p.Tags[:showCount] {
		tagS := r.NewStyle().Foreground(tagColor(t, theme))
		tagParts = append(tagParts, tagS.Render("["+t+"]"))
	}
	if len(tagParts) > 0 {
		for _, line := range strings.Split(wrapWidth(strings.Join(tagParts, " "), inner-2), "\n") {
			rightLines = append(rightLines, row(line))
		}
	}

	// Other projects list — capped so the panel can't outgrow the terminal.
	rightLines = append(rightLines, row(""))
	rightLines = append(rightLines, rule())
	rightLines = append(rightLines, row(dimStyle.Render("other projects")))
	shown := 0
	const maxOther = 6
	for i, op := range projects {
		if i == cursor || i >= projectsReveal { continue }
		if shown >= maxOther {
			rightLines = append(rightLines, row(dimStyle.Render(fmt.Sprintf("  … %d more", len(projects)-1-shown))))
			break
		}
		title := op.Title
		if maxT := inner - 9; maxT > 1 && len([]rune(title)) > maxT {
			title = string([]rune(title)[:maxT-1]) + "…"
		}
		dot := dimStyle.Render("·")
		rightLines = append(rightLines, row(dot+" "+dimStyle.Render(fmt.Sprintf("%02d", i+1)+". ")+dimMidStyle.Render(title)))
		shown++
	}
	rightLines = append(rightLines, boxBottom)

	// ── COMBINE — zip left and right lines side by side ────────────
	leftLines := strings.Split(left.String(), "\n")

	maxLines := len(leftLines)
	if len(rightLines) > maxLines { maxLines = len(rightLines) }
	for len(leftLines)  < maxLines { leftLines  = append(leftLines, "") }
	for len(rightLines) < maxLines { rightLines = append(rightLines, "") }

	sep := boxStyle.Render("│")
	var out strings.Builder
	out.WriteString("\n")
	for i := range leftLines {
		lLine := fitToWidth(r, leftLines[i], leftW)
		rLine := rightLines[i]
		out.WriteString(" "+lLine+" "+sep+" "+rLine+"\n")
	}
	return out.String()
}


