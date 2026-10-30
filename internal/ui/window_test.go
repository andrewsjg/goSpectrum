package ui

import (
	"testing"

	"go.spectrum/internal/machine"
)

func TestFitScale(t *testing.T) {
	cases := []struct{ w, h, want int }{
		{704, 592, 2},
		{800, 600, 2},
		{1408, 1184, 4},
		{352, 296, 1},
		{100, 100, 1},
		{2000, 296, 1},
		{900, 900, 2},
	}
	for _, c := range cases {
		if got := FitScale(c.w, c.h); got != c.want {
			t.Errorf("FitScale(%d,%d)=%d want %d", c.w, c.h, got, c.want)
		}
	}
}

func TestWindowSize(t *testing.T) {
	w, h := WindowSize(2)
	if w != 704 || h != 592 {
		t.Fatalf("scale 2 is %dx%d", w, h)
	}
	w, h = WindowSize(0)
	if w != machine.VisibleW || h != machine.VisibleH {
		t.Fatalf("scale 0 clamps to %dx%d", w, h)
	}
}

func TestTitle(t *testing.T) {
	if got := Title("", "", "", ""); got != "goSpectrum — BASIC — running" {
		t.Fatalf("%q", got)
	}
	if got := Title("game.tap", "paused", "ignored", ""); got != "goSpectrum — game.tap — paused" {
		t.Fatalf("%q", got)
	}
	if got := Title("x.sna", "error", "beyond", ""); got != "goSpectrum — x.sna — error: beyond" {
		t.Fatalf("%q", got)
	}
	if got := Title("", "running", "", "sound off"); got != "goSpectrum — BASIC — running — sound off" {
		t.Fatalf("%q", got)
	}
}

func TestFrameRGBA(t *testing.T) {
	var fr machine.Frame
	fr.Pix[0] = 9
	buf := FrameRGBA(fr)
	if len(buf) != machine.VisibleW*machine.VisibleH*4 {
		t.Fatalf("len %d", len(buf))
	}
	if buf[0] != 0 || buf[1] != 0 || buf[2] != 0xFF || buf[3] != 0xFF {
		t.Fatalf("bright blue %v", buf[:4])
	}
	if buf[4] != 0 || buf[5] != 0 || buf[6] != 0 || buf[7] != 0xFF {
		t.Fatalf("black %v", buf[4:8])
	}
}
