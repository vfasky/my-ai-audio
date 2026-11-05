package ui

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	waveBars   = 47 // odd, so the newest peak sits on the centre bar
	wavePitch  = 7
	waveBar    = 3.5
	waveHeight = 64
)

// waveLevels mirrors the newest peaks outward from the centre, as the
// reference's symmetric voice print: the centre is the newest frame and the
// edges are the oldest of the 24 most recent. Levels are perceptual (square
// root), taper toward the edges so the outermost bars settle into dots, and
// stay within [0, 1].
func waveLevels(bins [32]float64) [waveBars]float32 {
	var out [waveBars]float32
	mid := waveBars / 2
	for i := 0; i <= mid; i++ {
		level := math.Min(1, math.Sqrt(math.Max(0, bins[len(bins)-1-i]))*1.35)
		taper := 0.3 + 0.7*math.Cos(float64(i)/float64(mid)*math.Pi/2)
		v := float32(level * taper)
		out[mid-i], out[mid+i] = v, v
	}
	return out
}

// waveWidth is the drawn width of a well's glyph row.
func waveWidth() float32 { return (waveBars-1)*wavePitch + waveBar }

// drawWave paints live audio as rounded orange bars; while idle it paints the
// reference's row of dim dots.
func drawWave(p *ui.Painter, r ui.Rect, bins [32]float64, live bool) {
	x0 := r.X + (r.W-waveWidth())/2
	cy := r.Y + r.H/2
	if !live {
		for i := 0; i < waveBars; i += 2 {
			x := x0 + float32(i)*wavePitch
			p.Fill(ui.Rect{X: x, Y: cy - waveBar/2, W: waveBar, H: waveBar}, colIdleDot, waveBar/2)
		}
		return
	}
	levels := waveLevels(bins)
	for i, v := range levels {
		h := max(waveBar, v*r.H)
		x := x0 + float32(i)*wavePitch
		p.Fill(ui.Rect{X: x, Y: cy - h/2, W: waveBar, H: h}, colOrangeDeep.Mix(colOrange, 0.45+0.55*v), waveBar/2)
	}
}

// drawPulse paints the text well: dim dots at rest, an orange swell that
// travels across them while recognition or refinement is running.
func drawPulse(p *ui.Painter, r ui.Rect, active bool) {
	x0 := r.X + (r.W-waveWidth())/2
	cy := r.Y + r.H/2
	head := float32(-100)
	if active {
		p.AnimationFrame()
		const period = 1400 * time.Millisecond
		phase := float32(p.Now().UnixNano()%int64(period)) / float32(period)
		head = phase*(waveBars+8) - 4
	}
	for i := 0; i < waveBars; i += 2 {
		a := max(0, 1-abs32(float32(i)-head)/5)
		h := waveBar + a*20
		color := colIdleDot
		if active {
			color = colOrangeDeep.Mix(colOrange, a)
		}
		x := x0 + float32(i)*wavePitch
		p.Fill(ui.Rect{X: x, Y: cy - h/2, W: waveBar, H: h}, color, waveBar/2)
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// well is the single inset black tile holding the glyph row.
func well(c *ui.Context, lit bool, paint func(p *ui.Painter, r ui.Rect)) {
	ui.Column(c.Key("well")).Height(160).Radius(22).Background(colWell).
		Border(1, ui.RGBA(255, 255, 255, 0.04)).Center().Children(func() {
		ui.Box(c).Size(waveWidth()+8, waveHeight).Draw(paint)
	})
}
