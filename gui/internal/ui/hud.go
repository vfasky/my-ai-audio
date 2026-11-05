package ui

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// HUD window metrics: the capsule sits at the bottom centre, with the
// preview of the text above it and room for the cancel button's tooltip.
const (
	HUDWidth  = 510
	HUDHeight = 172
	// previewTail is the runes of transcript the preview keeps, newest
	// last: about three lines of Chinese.
	previewTail = 90
	miniBars    = 11
	// tickerWidth is the line of the capsule that shows the words being
	// spoken; tickerTail the runes it lays out, more than ever fit.
	tickerWidth = 280
	tickerTail  = 60
)

// tailText keeps the newest max runes, as a dictation bar shows the words
// being spoken rather than the first ones.
func tailText(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return "…" + string(r[len(r)-max:])
}

// miniLevels samples the mirrored waveform for the capsule's short bars.
func miniLevels(bins [32]float64) [miniBars]float32 {
	full := waveLevels(bins)
	var out [miniBars]float32
	mid, step := waveBars/2, 3
	for i := range out {
		out[i] = full[mid+(i-miniBars/2)*step]
	}
	return out
}

func drawMini(p *ui.Painter, r ui.Rect, bins [32]float64, phase string) {
	const pitch, bar = 3.375, 1.875
	x0 := r.X + (r.W-(miniBars-1)*pitch-bar)/2
	cy := r.Y + r.H/2
	live, working := isLive(phase), isWorking(phase) || phase == "finalizing"
	levels := miniLevels(bins)
	head := float32(-100)
	if working {
		p.AnimationFrame()
		const period = 1100 * time.Millisecond
		head = float32(p.Now().UnixNano()%int64(period))/float32(period)*(miniBars+4) - 2
	}
	for i := 0; i < miniBars; i++ {
		h, color := float32(bar), colIdleDot
		switch {
		case live:
			h = max(bar, levels[i]*r.H)
			color = colOrangeDeep.Mix(colOrange, 0.45+0.55*levels[i])
		case working:
			a := max(0, 1-abs32(float32(i)-head)/2.5)
			h = bar + a*7.5
			color = colOrangeDeep.Mix(colOrange, a)
		}
		x := x0 + float32(i)*pitch
		p.Fill(ui.Rect{X: x, Y: cy - h/2, W: bar, H: h}, color, bar/2)
	}
}

// hudLine is what the capsule says in each phase while it shows no words;
// hudWords are the words.
func hudLine(s Snapshot) string {
	switch s.Phase {
	case "starting":
		return "正在准备…"
	case "recording", "tail":
		return "请说话…"
	case "finalizing":
		return "正在识别…"
	case "refining":
		return "AI 润色中…  可直接插入原文"
	}
	if s.Status != "" {
		return s.Status
	}
	return "等待语音"
}

// hudWords are what was recognized so far, which the capsule shows in
// place of its line while the user speaks and recognition finishes.
func hudWords(s Snapshot) string {
	switch s.Phase {
	case "recording", "tail", "finalizing":
		return s.Partial
	}
	return ""
}

// hudPreview is the text the preview above the capsule shows: while
// refinement runs, the ASR original that the confirm button would insert.
func hudPreview(s Snapshot) string {
	if s.Phase == "refining" {
		return s.Original
	}
	return ""
}

// learnedLine names the words a dictation added to the vocabulary.
func learnedLine(words []string) string {
	if len(words) == 0 {
		return ""
	}
	return "「" + strings.Join(words, "」「") + "」已加入词库"
}

// learned is the capsule after an insertion added new words: a check, the
// words, and a button that undoes it.
func (v *View) learned(c *ui.Context, words []string) {
	ui.Row(c).Attach(ui.AnchorBottom, ui.AnchorBottom).Bottom(10.5).Height(39).Padding(5.25, 5.25, 5.25, 12).Gap(9).
		Radius(19.5).Background(colHUD).Border(1, ui.RGBA(255, 255, 255, 0.12)).Children(func() {
		ui.Icon(c, iconCheck).Size(13.5, 13.5).TextColor(colGreen)
		ui.Text(c, learnedLine(words)).FontSize(12.5).FontWeight(500).SingleLine().MaxWidth(352.5).TextColor(colText)
		undo := ui.ButtonBase(c.Key("hud-undo-learn")).Height(28.5).PaddingX(12).Radius(14.25).Center().Label("撤销")
		undo.Background(pick(undo.Hovered(), colPillHover, colPill))
		undo.Children(func() { ui.Text(c, "撤销").FontSize(12).FontWeight(500).TextColor(colSubtle).SingleLine() })
		if undo.Clicked() {
			v.Actions.UndoLearned()
		}
	})
}

// slide eases the shown width of the ticker toward the width of its words,
// so new words move in from the right edge rather than jump.
type slide struct {
	shown float32
	at    time.Time
}

// slideAfter: frames further apart than this start a new line, shown whole.
const slideAfter = time.Second

func (s *slide) step(w float32, now time.Time) float32 {
	dt := now.Sub(s.at)
	s.at = now
	if dt <= 0 || dt > slideAfter {
		s.shown = w
		return w
	}
	const tau = 70 * time.Millisecond
	s.shown += (w - s.shown) * float32(1-math.Exp(-float64(dt)/float64(tau)))
	if d := w - s.shown; d < 0.5 && d > -0.5 {
		s.shown = w
	}
	return s.shown
}

// ticker shows words on one line of fixed width: the newest at the right
// edge, sliding in as they arrive, the older scrolling out to the left
// under a fade.
func (v *View) ticker(c *ui.Context, words string) {
	text := tailText(words, tickerTail)
	ui.Box(c.Key("hud-ticker")).Size(tickerWidth, 19.5).Label(text).Draw(func(p *ui.Painter, r ui.Rect) {
		span := ui.Span{Text: text, Size: 12, Weight: 500, Color: colText}
		w, h := p.MeasureText(0, span)
		shown := v.slide.step(w, p.Now())
		if shown != w {
			p.AnimationFrame()
		}
		p.Clip(r, 0, func() {
			p.RichText(r.X+r.W-shown, r.Y+(r.H-h)/2, 0, span)
		})
		if shown > r.W {
			p.FillGradient(ui.Rect{X: r.X, Y: r.Y, W: 24, H: r.H}, ui.LinearGradient{From: colHUD, To: colHUD.Alpha(0), Angle: 90}, 0)
		}
	})
}

// DrawHUD renders the floating dictation capsule. It shares the snapshot of
// the main window; the app shows its panel without taking the keyboard.
func (v *View) DrawHUD(c *ui.Context) {
	c.SetTheme(theme)
	c.Root().Background(ui.Transparent)
	s := v.Snapshot
	ui.Box(c).Fill().Children(func() {
		if len(s.Learned) > 0 && !s.SpeechActive {
			v.learned(c, s.Learned)
			return
		}
		if text := hudPreview(s); text != "" {
			ui.Box(c).Attach(ui.AnchorBottom, ui.AnchorBottom).Bottom(57).MaxWidth(HUDWidth-30).Padding(9, 15).
				Radius(16).Background(colHUD).Border(1, ui.RGBA(255, 255, 255, 0.12)).Children(func() {
				ui.Text(c, tailText(text, previewTail)).FontSize(13.5).LineHeight(1.45).MaxLines(3).TextColor(colText)
			})
		}
		capsule := ui.Row(c).Attach(ui.AnchorBottom, ui.AnchorBottom).Bottom(10.5).Height(39).Padding(5.25).Gap(7.5).
			Radius(19.5).Background(colHUD).Border(1, ui.RGBA(255, 255, 255, 0.12))
		capsule.Children(func() {
			cancel := ui.ButtonBase(c.Key("hud-cancel")).Size(28.5, 28.5).Radius(14.25).Center().Label("取消").Tooltip("取消  Esc").
				Border(1, ui.RGBA(255, 255, 255, 0.16)).TextColor(colSubtle)
			cancel.Background(pick(cancel.Hovered(), colPillHover, colPill))
			cancel.Children(func() { ui.Icon(c, iconClose).Size(11.25, 11.25) })
			if cancel.Clicked() && s.SpeechActive {
				v.Actions.Cancel()
			}
			phase := s.Phase
			bins := s.Meter.Bins
			ui.Box(c).Size(40.5, 19.5).Draw(func(p *ui.Painter, r ui.Rect) { drawMini(p, r, bins, phase) })
			if words := hudWords(s); words != "" {
				v.ticker(c, words)
			} else {
				ui.Text(c, hudLine(s)).FontSize(11.25).FontWeight(500).SingleLine().MaxWidth(352.5).
					TextColor(colMuted).PaddingX(1.5)
			}
			commit := ui.ButtonBase(c.Key("hud-commit")).Size(28.5, 28.5).Radius(14.25).Center().Label("立即插入原文").
				Tooltip("跳过 AI 润色，立即插入").Disabled(!s.CanCommit)
			if s.CanCommit {
				commit.Background(pick(commit.Hovered(), ui.Hex("#EBEBEF"), ui.Hex("#FFFFFF"))).TextColor(colCanvas)
			} else {
				commit.Background(colPill).TextColor(colFaint)
			}
			commit.Children(func() { ui.Icon(c, iconCheck).Size(12, 12) })
			if commit.Clicked() && s.CanCommit {
				v.Actions.Commit()
			}
		})
	})
}
