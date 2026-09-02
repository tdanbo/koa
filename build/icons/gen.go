//go:build ignore

// Command gen renders koa's "K" monogram icons (PRD §14).
//
// It writes a light-glyph and a dark-glyph variant so the tray icon reads on
// both light and dark OS tray backgrounds, plus the square app icon Wails uses
// to build the platform bundles.
//
// Run it from the repository root:
//
//	go run ./build/icons/gen.go
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	outputs := []struct {
		path  string
		size  int
		glyph color.NRGBA
		frame color.NRGBA
		fill  color.NRGBA
		// weight scales the glyph stroke. Tray icons render very small (often
		// downscaled further by the OS), so they need a bolder stroke than the
		// app icon to stay legible against a busy tray background.
		weight float64
	}{
		// Tray, light glyph: for dark tray backgrounds.
		{"build/tray-dark.png", 32, nrgba(0xdc, 0xde, 0xdb, 0xff), nrgba(0xdc, 0xde, 0xdb, 0x8c), transparent(), 1.6},
		// Tray, dark glyph: for light tray backgrounds.
		{"build/tray-light.png", 32, nrgba(0x24, 0x28, 0x26, 0xff), nrgba(0x24, 0x28, 0x26, 0x8c), transparent(), 1.6},
		// Application icon: the monogram on koa's own window surface.
		{"build/appicon.png", 512, nrgba(0xdc, 0xde, 0xdb, 0xff), nrgba(0xff, 0xff, 0xff, 0x2e), nrgba(0x16, 0x18, 0x19, 0xff), 1.0},
	}

	for _, out := range outputs {
		img := render(out.size, out.glyph, out.frame, out.fill, out.weight)
		if err := writePNG(out.path, img); err != nil {
			fail(err)
		}
		fmt.Println("wrote", out.path)
	}

	// Windows wants an .ico; a PNG-compressed entry is valid and keeps the
	// glyph crisp at the size the shell asks for.
	for _, pair := range [][2]string{
		{"build/tray-dark.png", "build/tray-dark.ico"},
		{"build/tray-light.png", "build/tray-light.ico"},
	} {
		if err := writeICO(pair[0], pair[1]); err != nil {
			fail(err)
		}
		fmt.Println("wrote", pair[1])
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "icon generation failed:", err)
	os.Exit(1)
}

func nrgba(r, g, b, a uint8) color.NRGBA { return color.NRGBA{R: r, G: g, B: b, A: a} }
func transparent() color.NRGBA           { return color.NRGBA{} }

// render draws the boxed "K" at the given size. The geometry is expressed in a
// 32-unit grid and scaled up, so every variant is the same shape. weight
// scales the glyph stroke independently of the frame, for variants (like the
// tray icon) that need a bolder glyph to stay legible when small.
func render(size int, glyph, frame, fill color.NRGBA, weight float64) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	if fill.A > 0 {
		draw.Draw(img, img.Bounds(), &image.Uniform{fill}, image.Point{}, draw.Src)
	}

	unit := float64(size) / 32
	stroke := unit * 2.6 * weight
	if stroke < 1 {
		stroke = 1
	}

	// The square frame, matching the bordered monogram in the UI reference.
	frameStroke := unit * 1.4
	if frameStroke < 1 {
		frameStroke = 1
	}
	rect(img, 2*unit, 2*unit, 30*unit, 30*unit, frameStroke, frame)

	// The K: a stem plus two diagonals meeting at its middle. The bounds are
	// nudged right so the glyph sits optically centred inside the frame.
	line(img, 11.6*unit, 9*unit, 11.6*unit, 23*unit, stroke, glyph)
	line(img, 11.6*unit, 16*unit, 21.4*unit, 9*unit, stroke, glyph)
	line(img, 11.6*unit, 16*unit, 21.4*unit, 23*unit, stroke, glyph)

	return img
}

// rect strokes an axis-aligned rectangle outline.
func rect(img *image.NRGBA, x0, y0, x1, y1, w float64, c color.NRGBA) {
	line(img, x0, y0, x1, y0, w, c)
	line(img, x1, y0, x1, y1, w, c)
	line(img, x1, y1, x0, y1, w, c)
	line(img, x0, y1, x0, y0, w, c)
}

// line draws an anti-aliased segment by sampling the distance from each pixel
// centre to the segment — enough fidelity for a monogram, with no font
// dependency.
func line(img *image.NRGBA, x0, y0, x1, y1, w float64, c color.NRGBA) {
	half := w / 2
	bounds := img.Bounds()
	minX := clampInt(int(min(x0, x1)-half-1), bounds.Min.X, bounds.Max.X)
	maxX := clampInt(int(max(x0, x1)+half+2), bounds.Min.X, bounds.Max.X)
	minY := clampInt(int(min(y0, y1)-half-1), bounds.Min.Y, bounds.Max.Y)
	maxY := clampInt(int(max(y0, y1)+half+2), bounds.Min.Y, bounds.Max.Y)

	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			d := distanceToSegment(float64(x)+0.5, float64(y)+0.5, x0, y0, x1, y1)
			// One pixel of feather keeps diagonals from looking ragged.
			alpha := clamp01(half + 0.5 - d)
			if alpha <= 0 {
				continue
			}
			blend(img, x, y, c, alpha)
		}
	}
}

func distanceToSegment(px, py, x0, y0, x1, y1 float64) float64 {
	dx, dy := x1-x0, y1-y0
	lengthSq := dx*dx + dy*dy
	t := 0.0
	if lengthSq > 0 {
		t = clamp01(((px-x0)*dx + (py-y0)*dy) / lengthSq)
	}
	cx, cy := x0+t*dx, y0+t*dy
	return hypot(px-cx, py-cy)
}

func blend(img *image.NRGBA, x, y int, c color.NRGBA, alpha float64) {
	src := c
	src.A = uint8(float64(c.A) * alpha)
	if src.A == 0 {
		return
	}
	dst := img.NRGBAAt(x, y)
	sa := float64(src.A) / 255
	da := float64(dst.A) / 255
	outA := sa + da*(1-sa)
	if outA <= 0 {
		img.SetNRGBA(x, y, color.NRGBA{})
		return
	}
	mix := func(s, d uint8) uint8 {
		return uint8((float64(s)*sa + float64(d)*da*(1-sa)) / outA)
	}
	img.SetNRGBA(x, y, color.NRGBA{
		R: mix(src.R, dst.R),
		G: mix(src.G, dst.G),
		B: mix(src.B, dst.B),
		A: uint8(outA * 255),
	})
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
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

func hypot(a, b float64) float64 {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	if a == 0 && b == 0 {
		return 0
	}
	// Newton's method converges in a couple of steps for our magnitudes.
	x := a + b
	for i := 0; i < 12; i++ {
		x = 0.5 * (x + (a*a+b*b)/x)
	}
	return x
}

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// writeICO wraps a PNG in a single-entry ICO container.
func writeICO(pngPath, icoPath string) error {
	body, err := os.ReadFile(pngPath)
	if err != nil {
		return err
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	// ICONDIR: reserved, type (1 = icon), image count.
	binary.Write(&buf, binary.LittleEndian, uint16(0))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	// ICONDIRENTRY: 0 means 256 for width and height.
	buf.WriteByte(byte(cfg.Width % 256))
	buf.WriteByte(byte(cfg.Height % 256))
	buf.WriteByte(0)                                    // palette colours
	buf.WriteByte(0)                                    // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))  // colour planes
	binary.Write(&buf, binary.LittleEndian, uint16(32)) // bits per pixel
	binary.Write(&buf, binary.LittleEndian, uint32(len(body)))
	binary.Write(&buf, binary.LittleEndian, uint32(22)) // offset past the header
	buf.Write(body)

	return os.WriteFile(icoPath, buf.Bytes(), 0o644)
}
