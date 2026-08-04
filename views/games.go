package views

import (
	"fmt"
	"math/rand"

	"github.com/charmbracelet/lipgloss"
)

// Game is a playable mini-game rendered into the terminal grid.
type Game interface {
	Name() string
	Resize(w, h int)
	// Key feeds a keypress; it reports whether the game consumed it.
	Key(k string) bool
	// Step advances one frame of game time.
	Step()
	Render(r *lipgloss.Renderer, theme Theme) string
	Score() int
	Over() bool
	Restart()
}

// ─────────────────────────── Snake ───────────────────────────────────────

type point struct{ X, Y int }

// SnakeGame is classic snake on the terminal grid. The board is inset from
// the terminal edges so the border never collides with the footer.
type SnakeGame struct {
	w, h    int
	body    []point // head first
	dir     point
	nextDir point
	food    point
	score   int
	over    bool
	paused  bool
	tick    int
	speed   int // frames between moves; lower is faster
	grew    bool
}

func NewSnakeGame(w, h int) *SnakeGame {
	g := &SnakeGame{}
	g.Resize(w, h)
	return g
}

func (g *SnakeGame) Name() string { return "snake" }
func (g *SnakeGame) Score() int   { return g.score }
func (g *SnakeGame) Over() bool   { return g.over }

func (g *SnakeGame) Resize(w, h int) {
	// Board is measured in cells; leave a margin for border + HUD.
	bw, bh := w-4, h-6
	if bw < 10 {
		bw = 10
	}
	if bh < 6 {
		bh = 6
	}
	g.w, g.h = bw, bh
	g.Restart()
}

func (g *SnakeGame) Restart() {
	cx, cy := g.w/2, g.h/2
	g.body = []point{{cx, cy}, {cx - 1, cy}, {cx - 2, cy}}
	g.dir = point{1, 0}
	g.nextDir = g.dir
	g.score = 0
	g.over = false
	g.paused = false
	g.tick = 0
	g.speed = 4
	g.placeFood()
}

func (g *SnakeGame) placeFood() {
	// Reject positions on the snake. The board is far larger than the snake
	// in practice, so a bounded retry is plenty.
	for i := 0; i < 400; i++ {
		p := point{rand.Intn(g.w), rand.Intn(g.h)}
		if !g.onSnake(p) {
			g.food = p
			return
		}
	}
	// Board essentially full — scan for any free cell.
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			if p := (point{x, y}); !g.onSnake(p) {
				g.food = p
				return
			}
		}
	}
}

func (g *SnakeGame) onSnake(p point) bool {
	for _, b := range g.body {
		if b == p {
			return true
		}
	}
	return false
}

func (g *SnakeGame) Key(k string) bool {
	switch k {
	case "up", "k":
		if g.dir.Y == 0 { // no instant 180° reversal
			g.nextDir = point{0, -1}
		}
	case "down", "j":
		if g.dir.Y == 0 {
			g.nextDir = point{0, 1}
		}
	case "left", "h":
		if g.dir.X == 0 {
			g.nextDir = point{-1, 0}
		}
	case "right", "l":
		if g.dir.X == 0 {
			g.nextDir = point{1, 0}
		}
	case "p":
		g.paused = !g.paused
	case "r":
		g.Restart()
	default:
		return false
	}
	return true
}

func (g *SnakeGame) Step() {
	if g.over || g.paused {
		return
	}
	g.tick++
	if g.tick%g.speed != 0 {
		return
	}
	g.dir = g.nextDir

	head := g.body[0]
	next := point{head.X + g.dir.X, head.Y + g.dir.Y}

	// Walls are fatal.
	if next.X < 0 || next.Y < 0 || next.X >= g.w || next.Y >= g.h {
		g.over = true
		return
	}
	// Self-collision is fatal, except the tail cell which is about to move
	// away (unless we just ate and the tail stays put).
	for i, b := range g.body {
		if b == next {
			if i == len(g.body)-1 && !g.grew {
				continue
			}
			g.over = true
			return
		}
	}

	g.body = append([]point{next}, g.body...)
	if next == g.food {
		g.score += 10
		g.grew = true
		g.placeFood()
		if g.speed > 1 && g.score%50 == 0 {
			g.speed-- // speed up every 5 pickups
		}
	} else {
		g.grew = false
		g.body = g.body[:len(g.body)-1]
	}
}

func (g *SnakeGame) Render(r *lipgloss.Renderer, theme Theme) string {
	c := newCanvas(g.w+2, g.h+4)
	drawBorder(c, 0, 0, g.w+2, g.h+2, lipgloss.Color(theme.BoxBorder))

	// Food
	c.setBold(g.food.X+1, g.food.Y+1, '◆', lipgloss.Color(theme.Accent))

	// Snake — head bright, body fading toward the tail.
	for i, b := range g.body {
		x, y := b.X+1, b.Y+1
		switch {
		case i == 0:
			c.setBold(x, y, '█', lipgloss.Color(theme.Primary))
		case i < 4:
			c.set(x, y, '▓', lipgloss.Color(theme.Success))
		case i < 10:
			c.set(x, y, '▒', lipgloss.Color(theme.DimMid))
		default:
			c.set(x, y, '░', lipgloss.Color(theme.Dim))
		}
	}

	hud := fmt.Sprintf(" score %d   len %d ", g.score, len(g.body))
	c.text(1, g.h+2, hud, lipgloss.Color(theme.Text))
	switch {
	case g.over:
		c.text(1, g.h+3, " game over — [r] restart · [esc] back ", lipgloss.Color(theme.Warning))
	case g.paused:
		c.text(1, g.h+3, " paused — [p] resume ", lipgloss.Color(theme.Accent))
	default:
		c.text(1, g.h+3, " hjkl/arrows · [p] pause · [r] restart · [esc] back ", lipgloss.Color(theme.VeryDim))
	}
	return c.render(r)
}

// ─────────────────────────── Tetris ──────────────────────────────────────

// tetromino shapes, each as 4 rotations of 4 cells.
var tetrominoes = [7][4][4]point{
	{ // I
		{{0, 1}, {1, 1}, {2, 1}, {3, 1}}, {{2, 0}, {2, 1}, {2, 2}, {2, 3}},
		{{0, 2}, {1, 2}, {2, 2}, {3, 2}}, {{1, 0}, {1, 1}, {1, 2}, {1, 3}},
	},
	{ // O
		{{1, 0}, {2, 0}, {1, 1}, {2, 1}}, {{1, 0}, {2, 0}, {1, 1}, {2, 1}},
		{{1, 0}, {2, 0}, {1, 1}, {2, 1}}, {{1, 0}, {2, 0}, {1, 1}, {2, 1}},
	},
	{ // T
		{{1, 0}, {0, 1}, {1, 1}, {2, 1}}, {{1, 0}, {1, 1}, {2, 1}, {1, 2}},
		{{0, 1}, {1, 1}, {2, 1}, {1, 2}}, {{1, 0}, {0, 1}, {1, 1}, {1, 2}},
	},
	{ // S
		{{1, 0}, {2, 0}, {0, 1}, {1, 1}}, {{1, 0}, {1, 1}, {2, 1}, {2, 2}},
		{{1, 1}, {2, 1}, {0, 2}, {1, 2}}, {{0, 0}, {0, 1}, {1, 1}, {1, 2}},
	},
	{ // Z
		{{0, 0}, {1, 0}, {1, 1}, {2, 1}}, {{2, 0}, {1, 1}, {2, 1}, {1, 2}},
		{{0, 1}, {1, 1}, {1, 2}, {2, 2}}, {{1, 0}, {0, 1}, {1, 1}, {0, 2}},
	},
	{ // J
		{{0, 0}, {0, 1}, {1, 1}, {2, 1}}, {{1, 0}, {2, 0}, {1, 1}, {1, 2}},
		{{0, 1}, {1, 1}, {2, 1}, {2, 2}}, {{1, 0}, {1, 1}, {0, 2}, {1, 2}},
	},
	{ // L
		{{2, 0}, {0, 1}, {1, 1}, {2, 1}}, {{1, 0}, {1, 1}, {1, 2}, {2, 2}},
		{{0, 1}, {1, 1}, {2, 1}, {0, 2}}, {{0, 0}, {1, 0}, {1, 1}, {1, 2}},
	},
}

// TetrisGame is a standard 10-wide well with gravity, rotation and line clears.
type TetrisGame struct {
	w, h    int
	grid    []uint8 // 0 empty, else piece index + 1
	piece   int
	rot     int
	px, py  int
	nextP   int
	score   int
	lines   int
	over    bool
	paused  bool
	tick    int
	speed   int
}

func NewTetrisGame(w, h int) *TetrisGame {
	g := &TetrisGame{}
	g.Resize(w, h)
	return g
}

func (g *TetrisGame) Name() string { return "tetris" }
func (g *TetrisGame) Score() int   { return g.score }
func (g *TetrisGame) Over() bool   { return g.over }

func (g *TetrisGame) Resize(w, h int) {
	g.w = 10 // standard well width
	bh := h - 6
	if bh < 8 {
		bh = 8
	}
	if bh > 22 {
		bh = 22
	}
	g.h = bh
	g.Restart()
}

func (g *TetrisGame) Restart() {
	g.grid = make([]uint8, g.w*g.h)
	g.score, g.lines = 0, 0
	g.over, g.paused = false, false
	g.tick, g.speed = 0, 10
	g.nextP = rand.Intn(len(tetrominoes))
	g.spawn()
}

func (g *TetrisGame) spawn() {
	g.piece = g.nextP
	g.nextP = rand.Intn(len(tetrominoes))
	g.rot = 0
	g.px, g.py = g.w/2-2, 0
	if g.collides(g.px, g.py, g.rot) {
		g.over = true
	}
}

// collides reports whether the piece would overlap a wall, the floor, or a
// settled block at the given position and rotation.
func (g *TetrisGame) collides(px, py, rot int) bool {
	for _, c := range tetrominoes[g.piece][rot&3] {
		x, y := px+c.X, py+c.Y
		if x < 0 || x >= g.w || y >= g.h {
			return true
		}
		if y >= 0 && g.grid[y*g.w+x] != 0 {
			return true
		}
	}
	return false
}

func (g *TetrisGame) lock() {
	for _, c := range tetrominoes[g.piece][g.rot&3] {
		x, y := g.px+c.X, g.py+c.Y
		if x >= 0 && y >= 0 && x < g.w && y < g.h {
			g.grid[y*g.w+x] = uint8(g.piece + 1)
		}
	}
	g.clearLines()
	g.spawn()
}

func (g *TetrisGame) clearLines() {
	cleared := 0
	for y := g.h - 1; y >= 0; y-- {
		full := true
		for x := 0; x < g.w; x++ {
			if g.grid[y*g.w+x] == 0 {
				full = false
				break
			}
		}
		if !full {
			continue
		}
		// Shift everything above down one row.
		copy(g.grid[g.w:(y+1)*g.w], g.grid[0:y*g.w])
		for x := 0; x < g.w; x++ {
			g.grid[x] = 0
		}
		cleared++
		y++ // re-test this row, it now holds what was above
	}
	if cleared > 0 {
		g.lines += cleared
		// Standard scoring: more lines at once is worth disproportionately more.
		g.score += [5]int{0, 100, 300, 500, 800}[cleared]
		if g.speed > 2 && g.lines/10 > (g.lines-cleared)/10 {
			g.speed--
		}
	}
}

func (g *TetrisGame) Key(k string) bool {
	if g.over {
		if k == "r" {
			g.Restart()
			return true
		}
		return false
	}
	switch k {
	case "left", "h":
		if !g.collides(g.px-1, g.py, g.rot) {
			g.px--
		}
	case "right", "l":
		if !g.collides(g.px+1, g.py, g.rot) {
			g.px++
		}
	case "down", "j":
		if !g.collides(g.px, g.py+1, g.rot) {
			g.py++
			g.score++
		}
	case "up", "k", "x":
		if !g.collides(g.px, g.py, g.rot+1) {
			g.rot = (g.rot + 1) & 3
		}
	case " ", "space":
		for !g.collides(g.px, g.py+1, g.rot) {
			g.py++
			g.score += 2
		}
		g.lock()
	case "p":
		g.paused = !g.paused
	case "r":
		g.Restart()
	default:
		return false
	}
	return true
}

func (g *TetrisGame) Step() {
	if g.over || g.paused {
		return
	}
	g.tick++
	if g.tick%g.speed != 0 {
		return
	}
	if g.collides(g.px, g.py+1, g.rot) {
		g.lock()
	} else {
		g.py++
	}
}

// pieceColor maps a piece index to a themed colour.
func pieceColor(p int, theme Theme) lipgloss.Color {
	switch p % 7 {
	case 0:
		return lipgloss.Color(theme.Primary)
	case 1:
		return lipgloss.Color(theme.Accent)
	case 2:
		return lipgloss.Color(theme.Secondary)
	case 3:
		return lipgloss.Color(theme.Success)
	case 4:
		return lipgloss.Color(theme.Warning)
	case 5:
		return lipgloss.Color(theme.Purple)
	default:
		return lipgloss.Color(theme.Text)
	}
}

func (g *TetrisGame) Render(r *lipgloss.Renderer, theme Theme) string {
	// Each cell is drawn 2 columns wide so blocks look square.
	boardW := g.w*2 + 2
	c := newCanvas(boardW+16, g.h+4)
	drawBorder(c, 0, 0, boardW, g.h+2, lipgloss.Color(theme.BoxBorder))

	// Settled blocks
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			if v := g.grid[y*g.w+x]; v != 0 {
				col := pieceColor(int(v)-1, theme)
				c.set(1+x*2, 1+y, '█', col)
				c.set(2+x*2, 1+y, '█', col)
			}
		}
	}

	// Falling piece
	if !g.over {
		col := pieceColor(g.piece, theme)
		for _, cell := range tetrominoes[g.piece][g.rot&3] {
			x, y := g.px+cell.X, g.py+cell.Y
			if y >= 0 {
				c.setBold(1+x*2, 1+y, '█', col)
				c.setBold(2+x*2, 1+y, '█', col)
			}
		}
	}

	// Side panel: next piece + score
	sx := boardW + 2
	c.text(sx, 1, "NEXT", lipgloss.Color(theme.Accent))
	nc := pieceColor(g.nextP, theme)
	for _, cell := range tetrominoes[g.nextP][0] {
		c.set(sx+cell.X*2, 3+cell.Y, '█', nc)
		c.set(sx+1+cell.X*2, 3+cell.Y, '█', nc)
	}
	c.text(sx, 8, fmt.Sprintf("SCORE %d", g.score), lipgloss.Color(theme.Text))
	c.text(sx, 9, fmt.Sprintf("LINES %d", g.lines), lipgloss.Color(theme.DimMid))

	switch {
	case g.over:
		c.text(1, g.h+2, " game over — [r] restart · [esc] back ", lipgloss.Color(theme.Warning))
	case g.paused:
		c.text(1, g.h+2, " paused — [p] resume ", lipgloss.Color(theme.Accent))
	default:
		c.text(1, g.h+2, " ←→ move · ↑ rotate · ↓ drop · space slam ", lipgloss.Color(theme.VeryDim))
		c.text(1, g.h+3, " [p] pause · [r] restart · [esc] back ", lipgloss.Color(theme.VeryDim))
	}
	return c.render(r)
}

// drawBorder outlines a w×h rectangle at (x0,y0) with rounded corners.
func drawBorder(c *canvas, x0, y0, w, h int, col lipgloss.Color) {
	if w < 2 || h < 2 {
		return
	}
	for x := 1; x < w-1; x++ {
		c.set(x0+x, y0, '─', col)
		c.set(x0+x, y0+h-1, '─', col)
	}
	for y := 1; y < h-1; y++ {
		c.set(x0, y0+y, '│', col)
		c.set(x0+w-1, y0+y, '│', col)
	}
	c.set(x0, y0, '╭', col)
	c.set(x0+w-1, y0, '╮', col)
	c.set(x0, y0+h-1, '╰', col)
	c.set(x0+w-1, y0+h-1, '╯', col)
}

// NewAllGames returns one instance of each mini-game.
func NewAllGames(w, h int) []Game {
	return []Game{NewSnakeGame(w, h), NewTetrisGame(w, h)}
}
