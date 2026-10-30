package ui

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// GlyphW is the advance of one Font character at scale 1.
const GlyphW = 8

// Font draws text with the Spectrum's 8×8 ROM character set. Without a
// charset it falls back to Ebiten's debug font, which ignores scale and color.
type Font struct {
	charset []byte
	atlas   *ebiten.Image
}

// NewFont takes the 768 bytes from machine.Charset. Any other length,
// including nil, gives the fallback font.
func NewFont(charset []byte) *Font {
	if len(charset) != 96*8 {
		charset = nil
	}
	return &Font{charset: charset}
}

// Text draws s with its top-left at (x, y), each glyph scale×8 pixels.
// Characters outside space to © draw as '?'.
func (f *Font) Text(dst *ebiten.Image, s string, x, y, scale int, c color.Color) {
	if f.charset == nil {
		ebitenutil.DebugPrintAt(dst, s, x, y-4)
		return
	}
	if f.atlas == nil {
		f.atlas = ebiten.NewImage(96*8, 8)
		f.atlas.WritePixels(atlasPixels(f.charset))
	}
	var op ebiten.DrawImageOptions
	op.ColorScale.ScaleWithColor(c)
	n := 0
	for _, r := range s {
		g := int(r) - 32
		if g < 0 || g >= 96 {
			g = '?' - 32
		}
		op.GeoM.Reset()
		op.GeoM.Scale(float64(scale), float64(scale))
		op.GeoM.Translate(float64(x+n*GlyphW*scale), float64(y))
		dst.DrawImage(f.atlas.SubImage(image.Rect(g*8, 0, g*8+8, 8)).(*ebiten.Image), &op)
		n++
	}
}

// atlasPixels lays the glyphs side by side as white-on-transparent RGBA.
// Bit 7 of each byte is the leftmost pixel.
func atlasPixels(charset []byte) []byte {
	const w = 96 * 8
	buf := make([]byte, w*8*4)
	for g := 0; g < 96; g++ {
		for row := 0; row < 8; row++ {
			bits := charset[g*8+row]
			for col := 0; col < 8; col++ {
				if bits&(0x80>>col) == 0 {
					continue
				}
				o := (row*w + g*8 + col) * 4
				buf[o], buf[o+1], buf[o+2], buf[o+3] = 0xFF, 0xFF, 0xFF, 0xFF
			}
		}
	}
	return buf
}
