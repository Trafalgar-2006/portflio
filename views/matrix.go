package views

import (
	"math/rand"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var matrixAlphabet = []rune{
	'ｦ', 'ｱ', 'ｳ', 'ｴ', 'ｵ', 'ｶ', 'ｷ', 'ｸ', 'ｹ', 'ｺ',
	'ｻ', 'ｼ', 'ｽ', 'ｾ', 'ｿ', 'ﾀ', 'ﾁ', 'ﾂ', 'ﾃ', 'ﾄ',
	'ﾅ', 'ﾆ', 'ﾇ', 'ﾈ', 'ﾉ', 'ﾊ', 'ﾋ', 'ﾌ', 'ﾍ', 'ﾎ',
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
}

// MatrixColumn is a single falling stream of characters
type MatrixColumn struct {
	Head       int     // current head row position
	Trail      int     // trail length
	Speed      int     // advance every N ticks
	TicksSince int     // ticks since last advance
	Chars      [60]rune // cached random chars
}

// NewMatrixColumns initialises rain columns for the terminal width
func NewMatrixColumns(width, height int) []MatrixColumn {
	cols := make([]MatrixColumn, width)
	for i := range cols {
		cols[i] = MatrixColumn{
			Head:  -rand.Intn(height + 5),
			Trail: 5 + rand.Intn(12),
			Speed: 1 + rand.Intn(3),
		}
		for j := range cols[i].Chars {
			cols[i].Chars[j] = matrixAlphabet[rand.Intn(len(matrixAlphabet))]
		}
	}
	return cols
}

// TickMatrixColumns advances rain by one tick
func TickMatrixColumns(cols []MatrixColumn, height int) []MatrixColumn {
	for i := range cols {
		cols[i].TicksSince++
		if cols[i].TicksSince >= cols[i].Speed {
			cols[i].TicksSince = 0
			cols[i].Head++
			// Randomly mutate a char in the stream
			j := rand.Intn(len(cols[i].Chars))
			cols[i].Chars[j] = matrixAlphabet[rand.Intn(len(matrixAlphabet))]
			// Reset if fully off-screen
			if cols[i].Head-cols[i].Trail > height {
				resetRange := height / 2 // guard: rand.Intn panics on n<=0 (tiny/zero terminal height)
				if resetRange < 1 {
					resetRange = 1
				}
				cols[i].Head = -rand.Intn(resetRange)
				cols[i].Trail = 5 + rand.Intn(12)
				cols[i].Speed = 1 + rand.Intn(3)
			}
		}
	}
	return cols
}

// ComputeNameCells returns (x,y)→rune for every non-space cell in the banner
func ComputeNameCells(nameX, nameY int, banner []string) map[[2]int]rune {
	cells := make(map[[2]int]rune)
	for row, line := range banner {
		for col, r := range []rune(line) {
			if r != ' ' {
				cells[[2]int{nameX + col, nameY + row}] = r
			}
		}
	}
	return cells
}

// NameX/Y helpers — centres the banner on the screen
func MatrixNameOrigin(width, height int) (int, int) {
	maxLineLen := 0
	for _, l := range nameBanner {
		if len([]rune(l)) > maxLineLen {
			maxLineLen = len([]rune(l))
		}
	}
	x := (width - maxLineLen) / 2
	if x < 0 {
		x = 0
	}
	y := (height - len(nameBanner)) / 2
	if y < 2 {
		y = 2
	}
	return x, y
}

type cellKind int

const (
	kindEmpty  cellKind = iota
	kindHead            // bright white
	kindBright          // bright green
	kindMid             // mid green
	kindDim             // dim green
	kindLocked          // cyan — name char
)

func cellKindAt(x, y int, cols []MatrixColumn, locked map[[2]int]rune, fade bool) (cellKind, rune) {
	if r, ok := locked[[2]int{x, y}]; ok {
		return kindLocked, r
	}
	if x >= len(cols) {
		return kindEmpty, ' '
	}
	col := cols[x]
	dist := col.Head - y
	// Head starts as far negative as -(height+4), which can exceed len(Chars)
	// on a tall terminal — adding len(Chars) once isn't enough to make the
	// remainder non-negative, so normalise explicitly.
	idx := (y + col.Head) % len(col.Chars)
	if idx < 0 {
		idx += len(col.Chars)
	}
	ch := col.Chars[idx]

	if fade {
		// During fade phase, rain is very dim
		if dist >= 0 && dist <= col.Trail {
			return kindDim, ch
		}
		return kindEmpty, ' '
	}
	switch {
	case dist == 0:
		return kindHead, ch
	case dist > 0 && dist <= 3:
		return kindBright, ch
	case dist > 3 && dist <= col.Trail/2:
		return kindMid, ch
	case dist > col.Trail/2 && dist <= col.Trail:
		return kindDim, ch
	default:
		return kindEmpty, ' '
	}
}

// RenderMatrix renders the full matrix rain frame
func RenderMatrix(r *lipgloss.Renderer, width, height int, cols []MatrixColumn, locked map[[2]int]rune, fade bool, theme Theme) string {
	headS   := r.NewStyle().Foreground(lipgloss.Color(theme.MatrixHead)).Bold(true)
	brightS := r.NewStyle().Foreground(lipgloss.Color(theme.MatrixBright)).Bold(true)
	midS    := r.NewStyle().Foreground(lipgloss.Color(theme.MatrixMid))
	dimS    := r.NewStyle().Foreground(lipgloss.Color(theme.MatrixDim))
	cyanS   := r.NewStyle().Foreground(lipgloss.Color(theme.MatrixLocked)).Bold(true)

	var b strings.Builder

	for y := 0; y < height; y++ {
		// Group consecutive same-kind segments for efficient ANSI output
		type seg struct {
			kind  cellKind
			chars strings.Builder
		}
		var segs []seg
		curKind := cellKind(-1)

		for x := 0; x < width; x++ {
			k, ch := cellKindAt(x, y, cols, locked, fade)
			if k != curKind {
				segs = append(segs, seg{kind: k})
				curKind = k
			}
			segs[len(segs)-1].chars.WriteRune(ch)
		}

		for _, s := range segs {
			t := s.chars.String()
			switch s.kind {
			case kindHead:
				b.WriteString(headS.Render(t))
			case kindBright:
				b.WriteString(brightS.Render(t))
			case kindMid:
				b.WriteString(midS.Render(t))
			case kindDim:
				b.WriteString(dimS.Render(t))
			case kindLocked:
				b.WriteString(cyanS.Render(t))
			default:
				b.WriteString(t) // spaces — no ANSI
			}
		}
		// No trailing newline after the final row: emitting height+1 lines
		// overflows the terminal and scrolls the frame.
		if y < height-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}
