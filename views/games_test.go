package views

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// Games must survive any terminal size and any key, and never render a frame
// taller than the space they were given.
func TestGamesNeverPanic(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	keys := []string{"up", "down", "left", "right", "h", "j", "k", "l", "x", " ", "p", "r", "esc", "zzz", ""}
	for _, sz := range [][2]int{{0, 0}, {1, 1}, {5, 5}, {20, 10}, {80, 24}, {200, 60}} {
		for _, g := range NewAllGames(sz[0], sz[1]) {
			g, sz := g, sz
			t.Run(g.Name(), func(t *testing.T) {
				defer func() {
					if rec := recover(); rec != nil {
						t.Fatalf("%s panicked at %dx%d: %v", g.Name(), sz[0], sz[1], rec)
					}
				}()
				for i := 0; i < 400; i++ {
					g.Key(keys[i%len(keys)])
					g.Step()
					g.Render(r, ThemeDracula)
				}
			})
		}
	}
}

// Resizing mid-game must not corrupt state.
func TestGamesSurviveResize(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	for _, g := range NewAllGames(80, 24) {
		g := g
		defer func() {
			if rec := recover(); rec != nil {
				t.Fatalf("%s panicked across resize: %v", g.Name(), rec)
			}
		}()
		for _, sz := range [][2]int{{120, 40}, {20, 8}, {1, 1}, {80, 24}} {
			g.Resize(sz[0], sz[1])
			for i := 0; i < 50; i++ {
				g.Step()
				g.Render(r, ThemeDracula)
			}
		}
	}
}

// Snake must die on a wall, and not on the tail cell it's vacating.
func TestSnakeWallCollision(t *testing.T) {
	g := NewSnakeGame(40, 20)
	// Drive right until it hits the right wall.
	for i := 0; i < 5000 && !g.Over(); i++ {
		g.Step()
	}
	if !g.Over() {
		t.Fatal("snake never hit the wall travelling right")
	}
}

// Reversing straight back into itself must be rejected, not fatal.
func TestSnakeNoInstantReversal(t *testing.T) {
	g := NewSnakeGame(40, 20)
	if g.dir != (point{1, 0}) {
		t.Fatalf("unexpected initial direction %v", g.dir)
	}
	g.Key("left") // directly backwards — must be ignored
	if g.nextDir == (point{-1, 0}) {
		t.Error("snake accepted an instant 180° reversal")
	}
	// Perpendicular turns are fine.
	g.Key("up")
	if g.nextDir != (point{0, -1}) {
		t.Errorf("perpendicular turn rejected: %v", g.nextDir)
	}
}

// Eating food must grow the snake and raise the score.
func TestSnakeEatingGrows(t *testing.T) {
	g := NewSnakeGame(40, 20)
	startLen := len(g.body)
	// Place the food directly ahead of the head.
	head := g.body[0]
	g.food = point{head.X + 1, head.Y}
	for i := 0; i < g.speed; i++ {
		g.Step()
	}
	if len(g.body) != startLen+1 {
		t.Errorf("snake length %d after eating, want %d", len(g.body), startLen+1)
	}
	if g.Score() == 0 {
		t.Error("score did not increase after eating")
	}
	if g.Over() {
		t.Error("snake died from eating")
	}
}

// Food must never spawn on top of the snake.
func TestSnakeFoodNeverOnBody(t *testing.T) {
	g := NewSnakeGame(30, 14)
	for i := 0; i < 300; i++ {
		g.placeFood()
		if g.onSnake(g.food) {
			t.Fatalf("food spawned on the snake at %v", g.food)
		}
	}
}

// Restart must fully reset state.
func TestSnakeRestart(t *testing.T) {
	g := NewSnakeGame(40, 20)
	for i := 0; i < 5000 && !g.Over(); i++ {
		g.Step()
	}
	g.Restart()
	if g.Over() || g.Score() != 0 || len(g.body) != 3 {
		t.Errorf("restart left stale state: over=%v score=%d len=%d", g.Over(), g.Score(), len(g.body))
	}
}

// Tetris pieces must stay inside the well through every rotation.
func TestTetrisPiecesStayInBounds(t *testing.T) {
	g := NewTetrisGame(80, 24)
	for p := range tetrominoes {
		for rot := 0; rot < 4; rot++ {
			for _, c := range tetrominoes[p][rot] {
				if c.X < 0 || c.X > 3 || c.Y < 0 || c.Y > 3 {
					t.Errorf("piece %d rot %d has out-of-box cell %v", p, rot, c)
				}
			}
			if n := len(tetrominoes[p][rot]); n != 4 {
				t.Errorf("piece %d rot %d has %d cells, want 4", p, rot, n)
			}
		}
	}
	// Slamming repeatedly must never place a block outside the grid.
	for i := 0; i < 200 && !g.Over(); i++ {
		g.Key(" ")
		g.Step()
	}
	for i, v := range g.grid {
		if v > 7 {
			t.Fatalf("grid cell %d holds invalid piece id %d", i, v)
		}
	}
}

// A full row must clear and score.
func TestTetrisLineClear(t *testing.T) {
	g := NewTetrisGame(80, 24)
	g.Restart()
	// Fill the bottom row completely.
	bottom := (g.h - 1) * g.w
	for x := 0; x < g.w; x++ {
		g.grid[bottom+x] = 1
	}
	before := g.score
	g.clearLines()

	if g.lines != 1 {
		t.Errorf("lines = %d, want 1", g.lines)
	}
	if g.score <= before {
		t.Errorf("score did not increase: %d -> %d", before, g.score)
	}
	for x := 0; x < g.w; x++ {
		if g.grid[bottom+x] != 0 {
			t.Fatalf("bottom row not cleared at x=%d", x)
		}
	}
}

// Clearing must shift the rows above down, not blank the board.
func TestTetrisClearShiftsRowsDown(t *testing.T) {
	g := NewTetrisGame(80, 24)
	g.Restart()
	bottom := (g.h - 1) * g.w
	above := (g.h - 2) * g.w
	for x := 0; x < g.w; x++ {
		g.grid[bottom+x] = 1 // full row — will clear
	}
	g.grid[above+3] = 5 // lone block above it
	g.clearLines()

	if g.grid[bottom+3] != 5 {
		t.Errorf("block above the cleared row did not fall: bottom[3]=%d, want 5", g.grid[bottom+3])
	}
	if g.grid[above+3] != 0 {
		t.Errorf("block was duplicated rather than moved")
	}
}

// Four rows at once must score more than four single rows.
func TestTetrisTetrisScoresMore(t *testing.T) {
	single := NewTetrisGame(80, 24)
	single.Restart()
	for i := 0; i < 4; i++ {
		row := (single.h - 1) * single.w
		for x := 0; x < single.w; x++ {
			single.grid[row+x] = 1
		}
		single.clearLines()
	}

	quad := NewTetrisGame(80, 24)
	quad.Restart()
	for y := quad.h - 4; y < quad.h; y++ {
		for x := 0; x < quad.w; x++ {
			quad.grid[y*quad.w+x] = 1
		}
	}
	quad.clearLines()

	if quad.score <= single.score {
		t.Errorf("four-at-once scored %d, four singles scored %d — should be higher", quad.score, single.score)
	}
	if quad.lines != 4 {
		t.Errorf("quad clear counted %d lines, want 4", quad.lines)
	}
}

// A well filled to the top must end the game rather than loop forever.
func TestTetrisGameOver(t *testing.T) {
	g := NewTetrisGame(80, 24)
	for i := 0; i < 20000 && !g.Over(); i++ {
		g.Step() // never move; pieces stack until they reach the top
	}
	if !g.Over() {
		t.Error("tetris never reached game over while stacking")
	}
	// Restart must clear it.
	g.Restart()
	if g.Over() {
		t.Error("restart left the game in a game-over state")
	}
}

// Games must render under every theme.
func TestGamesAllThemes(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	for _, th := range Themes {
		for _, g := range NewAllGames(80, 24) {
			for i := 0; i < 20; i++ {
				g.Step()
			}
			if out := g.Render(r, th); strings.TrimSpace(stripAnsi(out)) == "" {
				t.Errorf("%s rendered blank under theme %s", g.Name(), th.Name)
			}
		}
	}
}
