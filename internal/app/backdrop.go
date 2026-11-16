package app

import (
	"math"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"

	"lilt/internal/player"
)

// The backdrop of Now Playing paints the cover's colors behind it, as
// NetEase's own player does: a deep gradient of its main color, lighter at
// the top, with the cover itself, blurred, faintly through it. It stays
// still, so it is painted once a song, into a small canvas an eighth of the
// window across, which shows scaled up as smooth as it is: what moves over
// it then draws one image under it.
//
// The experimental fog (settings.Fog) makes it alive, as NetEase's mobile
// player's is: fog of the cover's color, billowing as it rises slowly
// through a deep shade of it, with specks of dust glinting in it, brighter
// and thicker with the bass. A shader paints it into a canvas, half the
// window across on the GPU, thirty times a second while the music plays;
// the CPU paints it still. Paused, the fog glides to a stop, and the
// canvas stays as it is.
//
// Another song's backdrop fades in over the last one's, which stays as it
// was, in a canvas of its own, until it is gone.

// fogSource paints the fog: noise of five octaves, warped by noise of
// its own so that it billows rather than slides, rising with Time, Fog
// over Deep where it is thick; thickest a little above the middle, and
// thinning toward the top and the controls at the foot.
const fogSource = `package main

var Time float
var Bass float
var Deep vec4
var Fog vec4

func hash(p vec2) float {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453)
}

func noise(p vec2) float {
	i := floor(p)
	f := fract(p)
	u := f * f * (3.0 - 2.0*f)
	a := hash(i)
	b := hash(i + vec2(1, 0))
	c := hash(i + vec2(0, 1))
	d := hash(i + vec2(1, 1))
	return mix(mix(a, b, u.x), mix(c, d, u.x), u.y)
}

func fbm(p vec2) float {
	v := 0.0
	amp := 0.5
	for i := 0; i < 5; i++ {
		v += amp * noise(p)
		p = p*2.03 + vec2(1.7, 9.2)
		amp *= 0.5
	}
	return v
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	p := pos / size.y * 2.4
	t := Time
	rise := vec2(0, t*0.1)
	q := vec2(fbm(p+rise+vec2(0, t*0.04)), fbm(p+rise+vec2(5.2, 1.3)-vec2(t*0.05, 0)))
	f := fbm(p + rise + 0.9*q)
	// Broad banks of it, and the puffs within them.
	bank := fbm(p*0.45 + rise*0.6 + vec2(3.7, 0))
	cloud := smoothstep(0.3-0.06*Bass, 0.78, f) * (0.45 + 0.75*smoothstep(0.3, 0.65, bank))
	// Thickest a little above the middle.
	m := smoothstep(-0.1, 0.4, uv.y) * (1.0 - 0.5*smoothstep(0.65, 1.0, uv.y))
	c := mix(Deep.rgb, Fog.rgb, min(cloud*m*(0.85+0.35*Bass), 1.0))
	// A broad glow behind it all, darker at the corners.
	g := length((uv - vec2(0.5, 0.42)) * vec2(1.0, 1.25))
	c *= 1.08 - 0.45*smoothstep(0.2, 0.85, g)
	// Specks of dust, rising with the fog, each glinting in its own time.
	cell := size.y / 22
	sp := (pos + vec2(0, t*cell*0.9)) / cell
	id := floor(sp)
	h := hash(id + vec2(3.1, 7.7))
	if h > 0.93 {
		at := vec2(hash(id+vec2(1.3, 2.9)), hash(id+vec2(8.1, 4.3)))*0.8 + 0.1
		d := length((fract(sp)-at)*cell) / max(cell*0.05, 0.7)
		glint := 0.5 + 0.5*sin(t*(1.5+2.0*h)+h*40.0)
		c += vec3(0.75, 0.82, 1.0) * exp(-d*d) * glint * (0.25 + 0.3*m)
	}
	return vec4(c, 1)
}
`

var fluidSource = `package main

var Time float
var Bass float
var A vec4
var B vec4

func hash(p vec2) float {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453)
}

func noise(p vec2) float {
	i := floor(p)
	f := fract(p)
	u := f * f * (3.0 - 2.0*f)
	a := hash(i)
	b := hash(i + vec2(1, 0))
	c := hash(i + vec2(0, 1))
	d := hash(i + vec2(1, 1))
	return mix(mix(a, b, u.x), mix(c, d, u.x), u.y)
}

// density is how much of a mass covers uv: one at its center, falling away
// smooth and plain as a bell does, with no shoulder for the eye to read as a
// ring. uv is the pixel over the canvas, asp the canvas' width over its
// height, radius the mass' share of the height.
func density(uv vec2, asp float, center vec2, radius float) float {
	d := length((uv - center) * vec2(asp, 1.0)) / radius
	return exp(-d*d*2.4)
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	asp := size.x / size.y
	t := Time * 0.85
	// The whole field drifts through noise of its own, so that the edges of
	// the colors fold and flow as liquids do, rather than sliding whole.
	warp := vec2(noise(uv*1.8+vec2(t*0.30, 0))-0.5, noise(uv*1.8+vec2(4.7, 1.3)-vec2(0, t*0.26))-0.5)
	p := uv + warp*0.34
	// The two colors folded into a deep base, drifting between them.
	base := mix(A.rgb, B.rgb, 0.5+0.35*sin(t*0.5)) * 0.8
	// Each color takes a mass far larger than the view, its center near a
	// corner: their edges fall outside the frame, so what shows is fields of
	// color meeting and folding, never a ball. They breathe as the bass comes.
	breath := 1.0 + 0.03*sin(t*0.8) + 0.06*Bass
	da := density(p, asp, vec2(0.20+0.12*sin(t*0.6), 0.26+0.10*sin(t*0.9+1.3)), 1.30*breath)
	db := density(p, asp, vec2(0.80+0.12*sin(t*0.7+2.1), 0.74+0.11*sin(t*0.5+0.6)), 1.20*breath)
	cov := 1.0 - (1.0-da)*(1.0-db)
	mixed := (A.rgb*da + B.rgb*db) / (da + db + 0.0001)
	c := mix(base, mixed*1.04, smoothstep(0.0, 1.0, cov))
	// A soft light through the middle, darker at the corners and the foot.
	glow := length((uv - vec2(0.5, 0.42)) * vec2(1.0, 1.25))
	c *= 1.16 - 0.5*smoothstep(0.15, 0.92, glow)
	c = mix(c, c*0.82, smoothstep(0.72, 1.0, uv.y))
	// A grain of noise over it, which keeps the slow gradients from banding
	// into rings where they are stretched over the screen.
	c += vec3(hash(pos+vec2(t*7.0, t*4.0)) - 0.5) * (2.0/255.0)
	return vec4(c, 1)
}
`

var fluidShader = sync.OnceValue(func() *shader.Shader { return shader.MustCompile(fluidSource) })

var fogShader = sync.OnceValue(func() *shader.Shader { return shader.MustCompile(fogSource) })

// backdropFrame is how often the fog is painted while it moves.
const backdropFrame = time.Second / 30

// backdropFade is how long another song's backdrop takes to fade in.
const backdropFade = 700 * time.Millisecond

// backdropCache is the backdrop as last painted, and where the fog's
// drift and its pulse are.
type backdropCache struct {
	canvas *ui.Canvas
	before *ui.Canvas // the song before's, fading out over it
	fadeAt time.Time  // when it began to
	art    *artwork
	tint   ui.Color
	second ui.Color  // the cover's second color, for the fluid
	style  string    // the effect painted
	at     time.Time // when it was painted
	drift  float64   // the clouds' time, in seconds, which stops on pause
	speed  float64   // how fast drift goes: 1 playing, 0 paused
	level  barState
	last   time.Time
}

// backdrop paints the backdrop behind Now Playing, in the effect chosen:
// the cover's colors as a gradient, the fluid of two of them, or the fog.
func (a *app) backdrop(c *ui.Context, art *artwork, tint ui.Color) {
	pl := a.pl
	style, still := a.settings.backdropStyle(), c.Preferences().ReduceMotion
	// The second color the fluid mixes in stays the last song's while the
	// next cover loads, as the first does.
	second := tint
	if art != nil && !art.failed {
		if c := art.accent; c.R+c.G+c.B > 0 {
			second = c
		}
	}
	// The effects that move: those painted afresh as the music plays.
	moves := style == backdropFluid || style == backdropFog
	ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Draw(func(p *ui.Painter, r ui.Rect) {
		// The CPU, drawing them for want of a GPU, paints far fewer
		// pixels, and those once a song: they stay still.
		gpu := p.Accelerated()
		div := float32(8)
		if moves && gpu {
			div = 2
			if style == backdropFluid {
				// The fluid is painted smaller still: its colors and their
				// shadows only show as they are, smooth over one another.
				div = 3
			}
		}
		cw, ch := int(r.W/div)+1, int(r.H/div)+1
		bd := &a.backdropCache
		if bd.canvas != nil {
			if w, h := bd.canvas.Size(); w != cw || h != ch {
				bd.canvas, bd.before = nil, nil
			}
		}
		now := p.Now()
		if bd.canvas != nil && (bd.art != art || bd.tint != tint) {
			// Another song: the last one's stays as it was, fading out.
			bd.canvas, bd.before, bd.fadeAt = bd.before, bd.canvas, now
		}
		moving := moves && gpu && !still
		lv, live := pl.Levels()
		moved := bd.step(now, pl.Playing() && moving, lv, live && moving)
		due := now.Sub(bd.at) >= backdropFrame-2*time.Millisecond
		if bd.canvas == nil || bd.art != art || bd.tint != tint || bd.second != second ||
			bd.style != style || moved && due {
			if bd.canvas == nil {
				bd.canvas = ui.NewCanvas(cw, ch)
			}
			bd.art, bd.tint, bd.second, bd.style, bd.at = art, tint, second, style, now
			bass := bd.level.group(0, 6)
			bd.canvas.Draw(p, func(cp *ui.Painter) {
				box := ui.Rect{W: float32(cw), H: float32(ch)}
				switch style {
				case backdropFog:
					drawFog(cp, box, tint, float32(bd.drift), bass)
				case backdropFluid:
					drawFluid(cp, box, tint, second, float32(bd.drift), bass)
				default:
					drawBackdrop(cp, box, art, tint)
				}
			})
		}
		p.Image(bd.canvas.Bitmap(), r, ui.FillBox)
		if bd.before != nil {
			if k := float32(now.Sub(bd.fadeAt)) / float32(backdropFade); k < 1 {
				// Eased out, as it fades.
				k = 1 - (1-k)*(1-k)*(1-k)
				p.ImageRegion(bd.before.Bitmap(), ui.Rect{}, r, ui.ImageOptions{Tint: white.Alpha(1 - k)})
				p.AnimationFrame()
			} else {
				bd.before = nil
			}
		}
		if moved {
			p.After(backdropFrame)
		}
	})
}

// step moves the drift and the pulse on to now, and reports whether
// either still moves.
func (bd *backdropCache) step(now time.Time, playing bool, lv [player.Bands]float32, live bool) bool {
	dt := now.Sub(bd.last).Seconds()
	if bd.last.IsZero() || dt > 0.2 {
		dt = 1.0 / 30
	}
	bd.last = now
	to := 0.0
	if playing {
		to = 1
	}
	// The drift eases in as the music plays, and glides to a stop.
	bd.speed += (to - bd.speed) * min(1, 2.5*dt)
	if math.Abs(to-bd.speed) < 0.005 {
		bd.speed = to
	}
	bd.drift += bd.speed * dt
	bd.level.step(now, lv, live)
	return bd.speed > 0 || bd.level.group(0, 6) > 0.004
}

// drawBackdrop paints the still backdrop in r.
func drawBackdrop(p *ui.Painter, r ui.Rect, art *artwork, tint ui.Color) {
	top, foot := backdropColors(tint)
	p.FillGradient(r, ui.LinearGradient{Angle: 180, Oklab: true, Stops: []ui.GradientStop{
		{Color: top, At: 0}, {Color: foot, At: 1},
	}}, 0)
	if art != nil && art.backdrop != nil {
		// The cover, blurred, as a faint wash of its own colors.
		p.ImageRegion(art.backdrop, ui.Rect{W: 48, H: 48}, coverRect(r, 48, 48), ui.ImageOptions{Tint: white.Alpha(0.1)})
	}
	// Darker toward the foot, under the controls.
	p.FillGradient(r, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
		{Color: radixInk.Alpha(0), At: 0.55}, {Color: radixInk.Alpha(0.18), At: 1},
	}}, 0)
}

// coverRect returns where an image of w×h goes to cover r, centered.
func coverRect(r ui.Rect, w, h float32) ui.Rect {
	s := max(r.W/w, r.H/h)
	return ui.Rect{X: r.X + (r.W-w*s)/2, Y: r.Y + (r.H-h*s)/2, W: w * s, H: h * s}
}

// backdropColors returns the top and the foot of the backdrop's gradient
// for a cover of tint: its hue, deep and rich, at the same depth whatever
// the cover, so that white reads over it, as NetEase's player paints it.
// A gray cover stays gray.
func backdropColors(tint ui.Color) (top, foot ui.Color) {
	h, s, _ := hsv(float64(tint.R)/255, float64(tint.G)/255, float64(tint.B)/255)
	if s > 0.12 {
		s = min(max(s*1.3, 0.45), 0.66)
	}
	return hsvColor(h, s, 0.35), hsvColor(h, min(s*1.05, 0.7), 0.24)
}

// drawFog paints the fog of tint in r at drift seconds, lit by bass.
func drawFog(p *ui.Painter, r ui.Rect, tint ui.Color, drift, bass float32) {
	deep, fog := fogColors(tint)
	fogShader().Draw(p, r, shader.Options{Uniforms: map[string]any{
		"Time": drift, "Bass": min(bass, 1), "Deep": deep, "Fog": fog,
	}})
}

// drawFluid paints the fluid of the cover's two colors in r at drift
// seconds, its masses swelling as the bass comes.
func drawFluid(p *ui.Painter, r ui.Rect, main, second ui.Color, drift, bass float32) {
	a, b := fluidColors(main, second)
	fluidShader().Draw(p, r, shader.Options{Uniforms: map[string]any{
		"Time": drift, "Bass": min(bass, 1), "A": a, "B": b,
	}})
}

// fluidColors returns the two colors the fluid mixes: the cover's vivid
// color and its second, deep and rich alike so that white reads over them.
// A cover of one hue has its second turned away, and a gray one keeps two
// shades of gray.
func fluidColors(main, second ui.Color) (a, b ui.Color) {
	h1, s1, _ := hsv(float64(main.R)/255, float64(main.G)/255, float64(main.B)/255)
	h2, s2, _ := hsv(float64(second.R)/255, float64(second.G)/255, float64(second.B)/255)
	if s1 < 0.12 && s2 < 0.12 {
		// Gray: two shades of it, so that the fluid still has a shape.
		return hsvColor(0, 0, 0.35), hsvColor(0, 0, 0.21)
	}
	depth := func(s float64) float64 { return min(max(s*1.25, 0.5), 0.88) }
	if hueGap(h1, h2) < 40 {
		// The same hue, or nearly: the second turns away from the first.
		h2, s2 = math.Mod(h1+150, 360), max(s2, s1*0.9)
	}
	if s1 < 0.12 {
		h1, s1 = h2, s2
	}
	return hsvColor(h1, depth(s1), 0.32), hsvColor(h2, depth(s2), 0.26)
}

// hueGap is how far apart two hues are, the short way round, in degrees.
func hueGap(a, b float64) float64 {
	d := math.Abs(math.Mod(a-b+540, 360) - 180)
	return 180 - d
}

// fogColors returns the deep shade behind the fog and the fog's own for a
// cover of tint: its hue, rich, at the same depths whatever the cover, so
// that white reads over them. A gray cover stays gray.
func fogColors(tint ui.Color) (deep, fog ui.Color) {
	h, s, _ := hsv(float64(tint.R)/255, float64(tint.G)/255, float64(tint.B)/255)
	if s > 0.12 {
		s = min(max(s*1.3, 0.6), 0.85)
	}
	return hsvColor(h, s, 0.3), hsvColor(h, s*0.6, 0.72)
}

// hsvColor is the color of hue h (degrees), saturation s and value v.
func hsvColor(h, s, v float64) ui.Color {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	var r, g, b float64
	switch {
	case h < 60:
		r, g = c, x
	case h < 120:
		r, g = x, c
	case h < 180:
		g, b = c, x
	case h < 240:
		g, b = x, c
	case h < 300:
		r, b = x, c
	default:
		r, b = c, x
	}
	m := v - c
	return ui.RGB(uint8((r+m)*255+0.5), uint8((g+m)*255+0.5), uint8((b+m)*255+0.5))
}

// backdropOver paints the backdrop again over band, a strip of win, the
// box the backdrop fills, as opaque at each height in band as alpha says
// for it, from 0 at its top to 1 at its foot: what moves under the strip
// fades into the backdrop with no edge, the very pixels around it where
// it is opaque.
func (a *app) backdropOver(p *ui.Painter, win, band ui.Rect, alpha func(t float32) float32) {
	bd := &a.backdropCache
	if bd.canvas == nil || win.W <= 0 || win.H <= 0 || band.H <= 0 {
		return
	}
	cw, ch := bd.canvas.Size()
	sx, sy := float32(cw)/win.W, float32(ch)/win.H
	// Its song's fading in over the song before's, as the backdrop.
	before := float32(0)
	if bd.before != nil {
		if k := float32(p.Now().Sub(bd.fadeAt)) / float32(backdropFade); k < 1 {
			before = (1 - k) * (1 - k) * (1 - k)
		}
	}
	const strip = 2
	for y := float32(0); y < band.H; y += strip {
		o := alpha((y + strip/2) / band.H)
		if o <= 0.004 {
			continue
		}
		dst := ui.Rect{X: band.X, Y: band.Y + y, W: band.W, H: min(strip, band.H-y)}
		src := ui.Rect{X: (dst.X - win.X) * sx, Y: (dst.Y - win.Y) * sy, W: dst.W * sx, H: dst.H * sy}
		p.ImageRegion(bd.canvas.Bitmap(), src, dst, ui.ImageOptions{Tint: white.Alpha(o)})
		if before > 0 {
			p.ImageRegion(bd.before.Bitmap(), src, dst, ui.ImageOptions{Tint: white.Alpha(o * before)})
		}
	}
}
