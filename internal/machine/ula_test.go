package machine

import "testing"

func TestContentionDelay(t *testing.T) {
	if Delay(14335, 0x4000) != 6 {
		t.Fatalf("14335: %d", Delay(14335, 0x4000))
	}
	if Delay(14341, 0x4000) != 0 || Delay(14342, 0x4000) != 0 {
		t.Fatalf("pattern tail %d %d", Delay(14341, 0x4000), Delay(14342, 0x4000))
	}
	want := []int{6, 5, 4, 3, 2, 1, 0, 0}
	for i, d := range want {
		if got := Delay(14335+i, 0x4000); got != d {
			t.Fatalf("pattern[%d]=%d", i, got)
		}
	}
	if Delay(14335+8, 0x7FFF) != 6 {
		t.Fatal("pattern repeats inside the 128 T window")
	}
	if Delay(14335+128, 0x4000) != 0 {
		t.Fatal("96 T idle after the window")
	}
	if Delay(14335+224, 0x4000) != 6 {
		t.Fatal("pattern restarts on the next paper line")
	}
	for _, tstate := range []int{14335, 14341, 14342, 14335 + 224} {
		if Delay(tstate, 0x8000) != 0 || Delay(tstate, 0x3FFF) != 0 {
			t.Fatalf("uncontended address waited at %d", tstate)
		}
	}
	if VisibleW != 352 || VisibleH != 296 {
		t.Fatalf("frame %dx%d", VisibleW, VisibleH)
	}
	if TStatesPerFrame != 69888 || 312*224 != 69888 {
		t.Fatal("frame is not 69888 T-states")
	}
}

func TestPaperAddr(t *testing.T) {
	addr, bit := PaperAddr(0, 0)
	if addr != 0x4000 || bit != 7 {
		t.Fatalf("0,0 -> %04X bit %d", addr, bit)
	}
	addr, bit = PaperAddr(7, 0)
	if addr != 0x4000 || bit != 0 {
		t.Fatalf("7,0 -> %04X bit %d", addr, bit)
	}
	addr, _ = PaperAddr(0, 1)
	if addr != 0x4100 {
		t.Fatalf("row 1 %04X", addr)
	}
	addr, _ = PaperAddr(0, 8)
	if addr != 0x4020 {
		t.Fatalf("row 8 %04X", addr)
	}
	addr, _ = PaperAddr(0, 64)
	if addr != 0x4800 {
		t.Fatalf("second third %04X", addr)
	}
	addr, _ = PaperAddr(0, 128)
	if addr != 0x5000 {
		t.Fatalf("third third %04X", addr)
	}
}

func TestFloatingBus(t *testing.T) {
	var mem Memory
	mem.RAM[0] = 0x5A
	mem.RAM[0x0100] = 0xA5 // 0x4100
	mem.RAM[0x1800] = 0x77 // 0x5800
	read := mem.Read
	if FloatingBus(14347, read) != 0x5A {
		t.Fatalf("bitmap %02X", FloatingBus(14347, read))
	}
	if FloatingBus(14348, read) != 0x77 {
		t.Fatalf("attr %02X", FloatingBus(14348, read))
	}
	if FloatingBus(14346, read) != 0xFF || FloatingBus(14351, read) != 0xFF {
		t.Fatal("idle floating bus")
	}
	if FloatingBus(14571, read) != 0xA5 {
		t.Fatalf("second row %02X", FloatingBus(14571, read))
	}
}

func TestPalette(t *testing.T) {
	if PaletteIndex(0, 0) != 0 || PaletteIndex(0, 1) != 0 {
		t.Fatal("black stays index 0")
	}
	if PaletteIndex(1, 0) != 1 || PaletteIndex(1, 1) != 9 {
		t.Fatal("bright ink")
	}
	if PaletteIndex(6, 1) != 14 {
		t.Fatal("bright yellow")
	}
	r, g, b := RGB(1)
	if r != 0 || g != 0 || b != 0xD7 {
		t.Fatalf("blue %02X %02X %02X", r, g, b)
	}
	r, g, b = RGB(9)
	if r != 0 || g != 0 || b != 0xFF {
		t.Fatalf("bright blue %02X %02X %02X", r, g, b)
	}
	r, g, b = RGB(7)
	if r != 0xD7 || g != 0xD7 || b != 0xD7 {
		t.Fatal("white")
	}
	r, g, b = RGB(8)
	if r != 0 || g != 0 || b != 0 {
		t.Fatal("index 8 is black and is not emitted for bright black")
	}
	r, g, b = RGB(14)
	if r != 0xFF || g != 0xFF || b != 0 {
		t.Fatal("bright yellow channels")
	}
}
