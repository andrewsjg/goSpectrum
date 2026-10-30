package ui

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"go.spectrum/internal/machine"
)

// FitScale is the largest integer scale of the 352×296 frame that fits.
// It is at least 1, so a tiny window still draws one unscaled frame.
func FitScale(winW, winH int) int {
	if winW < machine.VisibleW || winH < machine.VisibleH {
		return 1
	}
	s := winW / machine.VisibleW
	if h := winH / machine.VisibleH; h < s {
		s = h
	}
	if s < 1 {
		return 1
	}
	return s
}

// WindowSize is the initial window for an integer scale, minimum 1.
func WindowSize(scale int) (int, int) {
	if scale < 1 {
		scale = 1
	}
	return machine.VisibleW * scale, machine.VisibleH * scale
}

// Title is the FR-010 window title.
// An empty name is BASIC. errorText is appended only while status is error.
func Title(name, status, errorText, notice string) string {
	if name == "" {
		name = "BASIC"
	}
	if status == "" {
		status = "running"
	}
	t := fmt.Sprintf("goSpectrum — %s — %s", name, status)
	if status == "error" && errorText != "" {
		t += ": " + errorText
	}
	if notice != "" {
		t += " — " + notice
	}
	return t
}

// FrameRGBA expands palette indexes to opaque RGBA using the ULA levels.
func FrameRGBA(fr machine.Frame) []byte {
	buf := make([]byte, machine.VisibleW*machine.VisibleH*4)
	for i, idx := range fr.Pix {
		r, g, b := machine.RGB(idx)
		o := i * 4
		buf[o] = r
		buf[o+1] = g
		buf[o+2] = b
		buf[o+3] = 0xFF
	}
	return buf
}

// DrawFrame paints the frame with nearest-neighbor integer scale, centered
// on black. Paper pixels stay square.
func DrawFrame(screen *ebiten.Image, img *ebiten.Image, fr machine.Frame) {
	screen.Fill(color.Black)
	if img == nil {
		return
	}
	img.WritePixels(FrameRGBA(fr))
	b := screen.Bounds()
	s := FitScale(b.Dx(), b.Dy())
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(float64(s), float64(s))
	vw := machine.VisibleW * s
	vh := machine.VisibleH * s
	op.GeoM.Translate(float64(b.Dx()-vw)/2, float64(b.Dy()-vh)/2)
	screen.DrawImage(img, op)
}
