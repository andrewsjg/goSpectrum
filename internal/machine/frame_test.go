package machine

import (
	"os"
	"testing"
)

func TestGoldenCellsAndFlash(t *testing.T) {
	m := New()
	if err := m.LoadROM(jrROM()); err != nil {
		t.Fatal(err)
	}
	m.mem.RAM[0] = 0xFF
	m.mem.RAM[0x1800] = 0x41 // bright ink 1
	fr, _ := m.RunFrame()
	if fr.Paper(0, 0) != 9 {
		t.Fatalf("bright ink 1 -> %d", fr.Paper(0, 0))
	}
	m.mem.RAM[0x1800] = 0x01
	fr, _ = m.RunFrame()
	if fr.Paper(0, 0) != 1 {
		t.Fatalf("ink 1 -> %d", fr.Paper(0, 0))
	}
	m.mem.RAM[0x1800] = 0x40 // bright black
	fr, _ = m.RunFrame()
	if fr.Paper(0, 0) != 0 {
		t.Fatalf("bright black -> %d", fr.Paper(0, 0))
	}
	m.mem.RAM[0x1800] = 0x00
	fr, _ = m.RunFrame()
	if fr.Paper(0, 0) != 0 {
		t.Fatalf("black -> %d", fr.Paper(0, 0))
	}

	// One cell: left nibble ink 2, right nibble paper 6. The next cell stays black.
	m.mem.RAM[0] = 0xF0
	for _, addr := range []int{0x0100, 0x0200, 0x0300, 0x0400, 0x0500, 0x0600, 0x0700} {
		m.mem.RAM[addr] = 0xF0
	}
	m.mem.RAM[0x1800] = 0x32
	fr, _ = m.RunFrame()
	for y := 0; y < 8; y++ {
		for x := 0; x < 4; x++ {
			if fr.Paper(x, y) != 2 {
				t.Fatalf("ink pixel %d,%d = %d", x, y, fr.Paper(x, y))
			}
		}
		for x := 4; x < 8; x++ {
			if fr.Paper(x, y) != 6 {
				t.Fatalf("paper pixel %d,%d = %d", x, y, fr.Paper(x, y))
			}
		}
	}
	if fr.Paper(8, 0) != 0 || fr.Paper(0, 8) != 0 {
		t.Fatal("attribute clash leaked")
	}

	m.mem.RAM[0] = 0xFF
	m.mem.RAM[0x1800] = 0x91 // flash, paper 2, ink 1
	m.frameIndex = 0
	var last Frame
	for i := 0; i < 16; i++ {
		last, _ = m.RunFrame()
	}
	if last.Paper(0, 0) != 1 {
		t.Fatalf("frame 15 index %d", last.Paper(0, 0))
	}
	last, _ = m.RunFrame()
	if last.Paper(0, 0) != 2 {
		t.Fatalf("frame 16 index %d", last.Paper(0, 0))
	}
	for m.frameIndex < 32 {
		last, _ = m.RunFrame()
	}
	if last.Paper(0, 0) != 2 {
		t.Fatalf("frame 31 index %d", last.Paper(0, 0))
	}
	last, _ = m.RunFrame()
	if last.Paper(0, 0) != 1 {
		t.Fatalf("frame 32 index %d", last.Paper(0, 0))
	}
}

func TestBorderChangesOnTheBeam(t *testing.T) {
	rom := make([]byte, 16384)
	copy(rom[3576:], []byte{0x3E, 0x02, 0x06, 0x00, 0x0E, 0x00, 0xD3, 0xFE, 0x18, 0xFE})
	m := New()
	if err := m.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	fr, _ := m.RunFrame()
	if fr.At(0, 0) != 0 || fr.At(0, 47) != 0 || fr.At(304, 47) != 0 || fr.At(47, 48) != 0 {
		t.Fatalf("early border %d %d %d %d", fr.At(0, 0), fr.At(0, 47), fr.At(304, 47), fr.At(47, 48))
	}
	if fr.At(304, 48) != 2 || fr.At(351, 48) != 2 || fr.At(0, 100) != 2 {
		t.Fatalf("late border %d %d %d", fr.At(304, 48), fr.At(351, 48), fr.At(0, 100))
	}
}

func TestDisplayROM(t *testing.T) {
	rom, err := os.ReadFile("../../testdata/display.rom")
	if err != nil {
		t.Fatal(err)
	}
	m := New()
	if err := m.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	fr, _ := m.RunFrame()
	if fr.Paper(0, 0) != 14 || fr.Paper(8, 0) != 6 {
		t.Fatalf("cells %d %d", fr.Paper(0, 0), fr.Paper(8, 0))
	}
	seen := map[byte]bool{}
	for y := 0; y < VisibleH; y++ {
		for x := 0; x < VisibleW; x++ {
			if x < 48 || x >= 304 || y < 48 || y >= 240 {
				seen[fr.At(x, y)] = true
			}
		}
	}
	if !seen[1] || !seen[2] {
		t.Fatalf("animated border %#v", seen)
	}
}
