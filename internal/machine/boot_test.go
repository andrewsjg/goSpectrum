package machine

import (
	"os"
	"testing"
)

func TestBootFrame(t *testing.T) {
	rom, err := os.ReadFile("../../testdata/test.rom")
	if err != nil {
		t.Fatal(err)
	}
	if len(rom) != 16384 {
		t.Fatalf("test rom %d", len(rom))
	}
	want := []byte{0x3E, 0xFF, 0x32, 0x00, 0x40, 0x3E, 0x07, 0x32, 0x00, 0x58, 0x3E, 0x01, 0xD3, 0xFE, 0x18, 0xFE}
	for i, b := range want {
		if rom[i] != b {
			t.Fatalf("prefix byte %d = %02X", i, rom[i])
		}
	}
	m := New()
	if err := m.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	fr, samples := m.RunFrame()
	if fr.Paper(0, 0) != 7 {
		t.Fatalf("paper (0,0) index %d", fr.Paper(0, 0))
	}
	if fr.At(0, 0) != 1 || fr.At(351, 295) != 1 {
		t.Fatalf("border %d %d", fr.At(0, 0), fr.At(351, 295))
	}
	if len(samples) == 0 {
		t.Fatal("a non-turbo frame emits samples")
	}
	if m.FrameT() < TStatesPerFrame {
		t.Fatalf("frameT %d", m.FrameT())
	}
	if m.Status() != "running" {
		t.Fatal(m.Status())
	}
}
