package views

import (
	"math"
	"math/rand"

	"github.com/charmbracelet/lipgloss"
)

// ─────────────────────────── Fire ────────────────────────────────────────

// FireEffect is the demoscene heat-map fire: seed the bottom row white-hot,
// then each cell above averages its neighbours below minus a cooling factor.
type FireEffect struct {
	w, h int
	heat []float64
}

func NewFireEffect(w, h int) *FireEffect {
	f := &FireEffect{}
	f.Resize(w, h)
	return f
}

func (f *FireEffect) Name() string      { return "fire" }
func (f *FireEffect) Interact(x, y int) { f.ignite(x, y) }

func (f *FireEffect) Resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	f.w, f.h = w, h
	f.heat = make([]float64, w*h)
}

// ignite dumps heat at a point, so a click throws up a flame.
func (f *FireEffect) ignite(x, y int) {
	for dy := -1; dy <= 1; dy++ {
		for dx := -2; dx <= 2; dx++ {
			nx, ny := x+dx, y+dy
			if nx >= 0 && ny >= 0 && nx < f.w && ny < f.h {
				f.heat[ny*f.w+nx] = 1
			}
		}
	}
}

func (f *FireEffect) Step() {
	if f.w < 1 || f.h < 1 {
		return
	}
	// Seed the bottom row with a flickering fuel bed.
	bottom := (f.h - 1) * f.w
	for x := 0; x < f.w; x++ {
		if rand.Float64() < 0.82 {
			f.heat[bottom+x] = 0.75 + rand.Float64()*0.25
		} else {
			f.heat[bottom+x] = rand.Float64() * 0.35
		}
	}
	// Propagate upward: each cell averages the three below it, minus cooling.
	for y := 0; y < f.h-1; y++ {
		for x := 0; x < f.w; x++ {
			below := (y + 1) * f.w
			sum := f.heat[below+x] * 1.30
			n := 1.30
			if x > 0 {
				sum += f.heat[below+x-1]
				n++
			}
			if x < f.w-1 {
				sum += f.heat[below+x+1]
				n++
			}
			if y < f.h-2 {
				sum += f.heat[(y+2)*f.w+x]
				n++
			}
			v := sum/n - 0.018 - rand.Float64()*0.020
			f.heat[y*f.w+x] = clampF(v, 0, 1)
		}
	}
}

func (f *FireEffect) Render(r *lipgloss.Renderer, theme Theme) string {
	c := newCanvas(f.w, f.h)
	for y := 0; y < f.h; y++ {
		for x := 0; x < f.w; x++ {
			v := f.heat[y*f.w+x]
			if v <= 0.04 {
				continue
			}
			c.set(x, y, rampAt(v), heatRamp(v, theme))
		}
	}
	return c.render(r)
}

// ─────────────────────────── 3D wireframe ────────────────────────────────

// vec3 is a point in model space.
type vec3 struct{ X, Y, Z float64 }

// WireframeEffect projects a rotating 3D solid onto the terminal grid,
// shading by depth. Shape 0 is a torus (the classic "donut"), shape 1 a cube.
type WireframeEffect struct {
	w, h  int
	angA  float64
	angB  float64
	shape int
	zbuf  []float64
	ch    []rune
	depth []float64
}

func NewWireframeEffect(w, h int, shape int) *WireframeEffect {
	e := &WireframeEffect{shape: shape}
	e.Resize(w, h)
	return e
}

func (e *WireframeEffect) Name() string {
	if e.shape == 1 {
		return "wireframe cube"
	}
	return "torus"
}

// Interact nudges the rotation, so clicking spins the solid.
func (e *WireframeEffect) Interact(x, y int) {
	e.angA += 0.25
	e.angB += 0.15
}

func (e *WireframeEffect) Resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	e.w, e.h = w, h
	e.zbuf = make([]float64, w*h)
	e.ch = make([]rune, w*h)
	e.depth = make([]float64, w*h)
}

func (e *WireframeEffect) Step() {
	e.angA += 0.041
	e.angB += 0.023
}

// rotate applies the current X-then-Y rotation to a model-space vector.
// Used for both points and surface normals, so lighting stays consistent
// with the projection.
func (e *WireframeEffect) rotate(p vec3) vec3 {
	sinA, cosA := math.Sin(e.angA), math.Cos(e.angA)
	sinB, cosB := math.Sin(e.angB), math.Cos(e.angB)

	y1 := p.Y*cosA - p.Z*sinA
	z1 := p.Y*sinA + p.Z*cosA
	x2 := p.X*cosB + z1*sinB
	z2 := -p.X*sinB + z1*cosB
	return vec3{X: x2, Y: y1, Z: z2}
}

// project maps a model-space point to a screen cell plus inverse depth.
func (e *WireframeEffect) project(p vec3) (int, int, float64) {
	rp := e.rotate(p)

	const viewer = 5.0
	ooz := 1 / (rp.Z + viewer) // inverse depth; larger = nearer

	// Fit the solid inside the viewport on both axes. The model's extent is
	// ~1.85 units and inverse depth peaks near 0.55, so a larger scale than
	// this clips the top and bottom off the shape.
	scaleH := float64(e.h) * 0.45
	scaleW := float64(e.w) * 0.22
	scale := scaleH
	if scaleW < scale {
		scale = scaleW
	}
	// Terminal cells are ~2x taller than wide, so scale X twice as much.
	sx := int(float64(e.w)/2 + scale*2*ooz*rp.X)
	sy := int(float64(e.h)/2 - scale*ooz*rp.Y)
	return sx, sy, ooz
}

func (e *WireframeEffect) plot(p vec3, ch rune) {
	sx, sy, ooz := e.project(p)
	if sx < 0 || sy < 0 || sx >= e.w || sy >= e.h {
		return
	}
	i := sy*e.w + sx
	if ooz > e.zbuf[i] { // nearer than whatever is already here
		e.zbuf[i] = ooz
		e.ch[i] = ch
		e.depth[i] = ooz
	}
}

func (e *WireframeEffect) Render(r *lipgloss.Renderer, theme Theme) string {
	for i := range e.zbuf {
		e.zbuf[i], e.ch[i], e.depth[i] = 0, ' ', 0
	}

	switch e.shape {
	case 1: // cube — draw the 12 edges as point sets
		const s = 1.15
		corners := [8]vec3{
			{-s, -s, -s}, {s, -s, -s}, {s, s, -s}, {-s, s, -s},
			{-s, -s, s}, {s, -s, s}, {s, s, s}, {-s, s, s},
		}
		edges := [12][2]int{
			{0, 1}, {1, 2}, {2, 3}, {3, 0},
			{4, 5}, {5, 6}, {6, 7}, {7, 4},
			{0, 4}, {1, 5}, {2, 6}, {3, 7},
		}
		for _, ed := range edges {
			a, b := corners[ed[0]], corners[ed[1]]
			const steps = 46
			for i := 0; i <= steps; i++ {
				t := float64(i) / steps
				e.plot(vec3{
					a.X + (b.X-a.X)*t,
					a.Y + (b.Y-a.Y)*t,
					a.Z + (b.Z-a.Z)*t,
				}, '#')
			}
		}
	default: // torus
		const (
			r1 = 0.55 // tube radius
			r2 = 1.30 // ring radius
		)
		// Light comes from above/behind the viewer's left shoulder.
		light := vec3{X: 0, Y: 0.7071, Z: -0.7071}
		for theta := 0.0; theta < 2*math.Pi; theta += 0.06 {
			ct, st := math.Cos(theta), math.Sin(theta)
			for phi := 0.0; phi < 2*math.Pi; phi += 0.02 {
				cp, sp := math.Cos(phi), math.Sin(phi)
				circleX := r2 + r1*ct
				p := vec3{X: circleX * cp, Y: r1 * st, Z: circleX * sp}

				// True surface normal of the torus at (theta, phi), rotated
				// with the model so shading tracks the spin.
				n := e.rotate(vec3{X: ct * cp, Y: st, Z: ct * sp})
				lum := n.X*light.X + n.Y*light.Y + n.Z*light.Z // dot product
				if lum <= 0 {
					continue // face points away from the light
				}
				e.plot(p, rampAt(clampF(lum, 0.05, 0.99)))
			}
		}
	}

	c := newCanvas(e.w, e.h)
	// Normalise inverse depth across what was actually drawn.
	maxD := 0.0
	for _, d := range e.depth {
		if d > maxD {
			maxD = d
		}
	}
	if maxD == 0 {
		maxD = 1
	}
	for y := 0; y < e.h; y++ {
		for x := 0; x < e.w; x++ {
			i := y*e.w + x
			if e.ch[i] == ' ' || e.ch[i] == 0 {
				continue
			}
			near := e.depth[i] / maxD // 1 = nearest
			c.set(x, y, e.ch[i], depthRamp(1-near, theme))
		}
	}
	return c.render(r)
}

// ─────────────────────────── Game of Life ────────────────────────────────

// LifeEffect runs Conway's Game of Life. Stagnation is detected by population
// history and reseeded, so it never settles into a dead screen.
type LifeEffect struct {
	w, h   int
	cells  []bool
	next   []bool
	age    []uint8
	pop    []int
	gen    int
}

func NewLifeEffect(w, h int) *LifeEffect {
	e := &LifeEffect{}
	e.Resize(w, h)
	return e
}

func (e *LifeEffect) Name() string { return "game of life" }

func (e *LifeEffect) Resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	e.w, e.h = w, h
	e.cells = make([]bool, w*h)
	e.next = make([]bool, w*h)
	e.age = make([]uint8, w*h)
	e.seed()
}

func (e *LifeEffect) seed() {
	for i := range e.cells {
		e.cells[i] = rand.Float64() < 0.28
		e.age[i] = 0
	}
	e.pop = e.pop[:0]
	e.gen = 0
}

// Interact stamps a glider at the click point.
func (e *LifeEffect) Interact(x, y int) {
	glider := [][2]int{{1, 0}, {2, 1}, {0, 2}, {1, 2}, {2, 2}}
	for _, g := range glider {
		nx, ny := x+g[0], y+g[1]
		if nx >= 0 && ny >= 0 && nx < e.w && ny < e.h {
			e.cells[ny*e.w+nx] = true
		}
	}
}

func (e *LifeEffect) at(x, y int) int {
	// Toroidal wrap keeps gliders in play instead of losing them off-screen.
	x = ((x % e.w) + e.w) % e.w
	y = ((y % e.h) + e.h) % e.h
	if e.cells[y*e.w+x] {
		return 1
	}
	return 0
}

func (e *LifeEffect) Step() {
	if e.w < 3 || e.h < 3 {
		return
	}
	alive := 0
	for y := 0; y < e.h; y++ {
		for x := 0; x < e.w; x++ {
			n := e.at(x-1, y-1) + e.at(x, y-1) + e.at(x+1, y-1) +
				e.at(x-1, y) + e.at(x+1, y) +
				e.at(x-1, y+1) + e.at(x, y+1) + e.at(x+1, y+1)
			i := y*e.w + x
			live := e.cells[i]
			e.next[i] = n == 3 || (live && n == 2)
			if e.next[i] {
				alive++
				if e.age[i] < 255 {
					e.age[i]++
				}
			} else {
				e.age[i] = 0
			}
		}
	}
	e.cells, e.next = e.next, e.cells
	e.gen++

	// Reseed on extinction or a stalled population.
	e.pop = append(e.pop, alive)
	if len(e.pop) > 24 {
		e.pop = e.pop[1:]
	}
	if alive == 0 {
		e.seed()
		return
	}
	if len(e.pop) == 24 {
		same := true
		for _, p := range e.pop {
			if p != e.pop[0] {
				same = false
				break
			}
		}
		if same {
			e.seed()
		}
	}
}

func (e *LifeEffect) Render(r *lipgloss.Renderer, theme Theme) string {
	c := newCanvas(e.w, e.h)
	for y := 0; y < e.h; y++ {
		for x := 0; x < e.w; x++ {
			i := y*e.w + x
			if !e.cells[i] {
				continue
			}
			// Colour by how long the cell has survived: new cells are hot,
			// long-lived structures cool into the background.
			switch a := e.age[i]; {
			case a <= 1:
				c.setBold(x, y, '█', lipgloss.Color(theme.Text))
			case a <= 3:
				c.set(x, y, '▓', lipgloss.Color(theme.Primary))
			case a <= 8:
				c.set(x, y, '▒', lipgloss.Color(theme.Secondary))
			case a <= 20:
				c.set(x, y, '░', lipgloss.Color(theme.DimMid))
			default:
				c.set(x, y, '·', lipgloss.Color(theme.Dim))
			}
		}
	}
	return c.render(r)
}

// ─────────────────────────── Falling sand ────────────────────────────────

const (
	sandEmpty uint8 = iota
	sandGrain
	sandWall
)

// SandEffect is cell-based particulate gravity: grains fall, pile up, and
// slide off slopes. Clicking pours more sand in.
type SandEffect struct {
	w, h  int
	grid  []uint8
	hue   []uint8 // per-grain colour index, so piles look layered
	frame int
}

func NewSandEffect(w, h int) *SandEffect {
	e := &SandEffect{}
	e.Resize(w, h)
	return e
}

func (e *SandEffect) Name() string { return "falling sand" }

func (e *SandEffect) Resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	e.w, e.h = w, h
	e.grid = make([]uint8, w*h)
	e.hue = make([]uint8, w*h)

	// A couple of ledges so the sand has something to pile on and pour off.
	for _, frac := range []float64{0.45, 0.72} {
		y := int(float64(h) * frac)
		x0, x1 := w/6, w*5/6
		if frac > 0.5 {
			x0, x1 = w/3, w-w/8
		}
		for x := x0; x < x1; x++ {
			if y >= 0 && y < h && x >= 0 && x < w {
				e.grid[y*w+x] = sandWall
			}
		}
	}
}

// Interact pours a handful of sand at the click point.
func (e *SandEffect) Interact(x, y int) {
	for dy := -1; dy <= 1; dy++ {
		for dx := -2; dx <= 2; dx++ {
			nx, ny := x+dx, y+dy
			if nx >= 0 && ny >= 0 && nx < e.w && ny < e.h && e.grid[ny*e.w+nx] == sandEmpty {
				e.grid[ny*e.w+nx] = sandGrain
				e.hue[ny*e.w+nx] = uint8(rand.Intn(4))
			}
		}
	}
}

func (e *SandEffect) Step() {
	if e.w < 2 || e.h < 2 {
		return
	}
	e.frame++

	// Emit from a couple of taps at the top.
	for _, x := range []int{e.w / 3, e.w * 2 / 3} {
		if e.frame%2 == 0 && x >= 0 && x < e.w && e.grid[x] == sandEmpty {
			e.grid[x] = sandGrain
			e.hue[x] = uint8(rand.Intn(4))
		}
	}

	// Sweep bottom-up so a grain moves at most once per frame.
	for y := e.h - 2; y >= 0; y-- {
		// Alternate scan direction to stop piles leaning consistently one way.
		x0, x1, dx := 0, e.w, 1
		if e.frame%2 == 0 {
			x0, x1, dx = e.w-1, -1, -1
		}
		for x := x0; x != x1; x += dx {
			i := y*e.w + x
			if e.grid[i] != sandGrain {
				continue
			}
			below := (y+1)*e.w + x
			switch {
			case e.grid[below] == sandEmpty:
				e.grid[below], e.hue[below] = sandGrain, e.hue[i]
				e.grid[i], e.hue[i] = sandEmpty, 0
			default:
				// Try to slide diagonally off the pile.
				dirs := [2]int{-1, 1}
				if rand.Intn(2) == 0 {
					dirs = [2]int{1, -1}
				}
				for _, d := range dirs {
					nx := x + d
					if nx < 0 || nx >= e.w {
						continue
					}
					diag := (y+1)*e.w + nx
					if e.grid[diag] == sandEmpty {
						e.grid[diag], e.hue[diag] = sandGrain, e.hue[i]
						e.grid[i], e.hue[i] = sandEmpty, 0
						break
					}
				}
			}
		}
	}

	// Drain the floor so the screen doesn't fill permanently.
	floor := (e.h - 1) * e.w
	for x := 0; x < e.w; x++ {
		if e.grid[floor+x] == sandGrain && rand.Float64() < 0.06 {
			e.grid[floor+x] = sandEmpty
		}
	}
}

func (e *SandEffect) Render(r *lipgloss.Renderer, theme Theme) string {
	c := newCanvas(e.w, e.h)
	grainCols := []lipgloss.Color{
		lipgloss.Color(theme.Accent),
		lipgloss.Color(theme.Warning),
		lipgloss.Color(theme.Secondary),
		lipgloss.Color(theme.Primary),
	}
	grainChars := []rune{'▪', '▫', '·', '▪'}
	for y := 0; y < e.h; y++ {
		for x := 0; x < e.w; x++ {
			i := y*e.w + x
			switch e.grid[i] {
			case sandGrain:
				h := int(e.hue[i]) % len(grainCols)
				c.set(x, y, grainChars[h], grainCols[h])
			case sandWall:
				c.set(x, y, '─', lipgloss.Color(theme.BoxBorder))
			}
		}
	}
	return c.render(r)
}

// ─────────────────────────── Tunnel ──────────────────────────────────────

// TunnelEffect is the demoscene warp tunnel: each cell's colour comes from its
// polar coordinates, with depth scrolling toward the viewer.
type TunnelEffect struct {
	w, h  int
	t     float64
	cx    float64
	cy    float64
}

func NewTunnelEffect(w, h int) *TunnelEffect {
	e := &TunnelEffect{}
	e.Resize(w, h)
	return e
}

func (e *TunnelEffect) Name() string { return "tunnel" }

func (e *TunnelEffect) Resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	e.w, e.h = w, h
	e.cx, e.cy = float64(w)/2, float64(h)/2
}

// Interact steers the tunnel's vanishing point toward the click.
func (e *TunnelEffect) Interact(x, y int) {
	e.cx, e.cy = float64(x), float64(y)
}

func (e *TunnelEffect) Step() { e.t += 0.085 }

func (e *TunnelEffect) Render(r *lipgloss.Renderer, theme Theme) string {
	c := newCanvas(e.w, e.h)
	for y := 0; y < e.h; y++ {
		for x := 0; x < e.w; x++ {
			// Correct for cells being about twice as tall as wide.
			dx := (float64(x) - e.cx) * 0.5
			dy := float64(y) - e.cy
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist < 0.6 {
				c.setBold(x, y, '@', lipgloss.Color(theme.Text))
				continue
			}
			ang := math.Atan2(dy, dx)
			depth := 8.0/dist + e.t          // scrolls toward the viewer
			twist := ang/math.Pi*4 + depth*0.4

			// Checkerboard the tunnel wall from depth and angle.
			v := math.Mod(math.Abs(math.Floor(depth)+math.Floor(twist)), 2)
			shade := clampF(1.4/dist, 0, 1)
			if v < 1 {
				shade *= 0.45
			}
			if shade < 0.05 {
				continue
			}
			c.set(x, y, rampAt(shade), depthRamp(1-shade, theme))
		}
	}
	return c.render(r)
}

// ─────────────────────────── Plasma ──────────────────────────────────────

// PlasmaEffect is the "bubble universe" — layered sine fields interfering to
// make a slowly churning organic field.
type PlasmaEffect struct {
	w, h int
	t    float64
	warp float64
}

func NewPlasmaEffect(w, h int) *PlasmaEffect {
	e := &PlasmaEffect{}
	e.Resize(w, h)
	return e
}

func (e *PlasmaEffect) Name() string      { return "plasma" }
func (e *PlasmaEffect) Interact(x, y int) { e.warp += 0.6 }

func (e *PlasmaEffect) Resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	e.w, e.h = w, h
}

func (e *PlasmaEffect) Step() { e.t += 0.062 }

func (e *PlasmaEffect) Render(r *lipgloss.Renderer, theme Theme) string {
	c := newCanvas(e.w, e.h)
	for y := 0; y < e.h; y++ {
		fy := float64(y)
		for x := 0; x < e.w; x++ {
			fx := float64(x) * 0.5 // aspect correction

			v := math.Sin(fx*0.18 + e.t)
			v += math.Sin((fy*0.24 + e.t*0.8))
			v += math.Sin((fx+fy)*0.14 + e.t*1.3)
			v += math.Sin(math.Sqrt(fx*fx+fy*fy)*0.22 + e.t*0.7 + e.warp)
			v = (v + 4) / 8 // normalise the four-wave sum into 0..1

			c.set(x, y, rampAt(v), heatRamp(v, theme))
		}
	}
	if e.warp > 0 {
		e.warp *= 0.94
	}
	return c.render(r)
}

// ─────────────────────────── registry ────────────────────────────────────

// NewAllEffects builds one instance of every screen effect, in screensaver
// cycle order.
func NewAllEffects(w, h int) []Effect {
	return []Effect{
		NewMatrixEffect(w, h),
		NewFireEffect(w, h),
		NewWireframeEffect(w, h, 0),
		NewPlasmaEffect(w, h),
		NewTunnelEffect(w, h),
		NewLifeEffect(w, h),
		NewSandEffect(w, h),
		NewWireframeEffect(w, h, 1),
	}
}

// MatrixEffect wraps the existing rain columns in the Effect interface so the
// screensaver can cycle it alongside the new effects.
type MatrixEffect struct {
	w, h int
	cols []MatrixColumn
}

func NewMatrixEffect(w, h int) *MatrixEffect {
	e := &MatrixEffect{}
	e.Resize(w, h)
	return e
}

func (e *MatrixEffect) Name() string { return "digital rain" }

func (e *MatrixEffect) Resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	e.w, e.h = w, h
	e.cols = NewMatrixColumns(w, h)
}

func (e *MatrixEffect) Step() { e.cols = TickMatrixColumns(e.cols, e.h) }

// Interact restarts the column under the cursor from the top.
func (e *MatrixEffect) Interact(x, y int) {
	if x >= 0 && x < len(e.cols) {
		e.cols[x].Head = 0
		e.cols[x].Trail = 6 + rand.Intn(12)
	}
}

func (e *MatrixEffect) Render(r *lipgloss.Renderer, theme Theme) string {
	return RenderMatrix(r, e.w, e.h, e.cols, map[[2]int]rune{}, false, theme)
}
