package machine

import "testing"

func TestMemoryMap(t *testing.T) {
	var m Memory
	if m.Read(0) != 0 || m.Read(0xFFFF) != 0 {
		t.Fatal("power-on RAM and ROM are not zero")
	}
	m.ROM[0] = 0xC3
	m.ROM[0x3FFF] = 0x11
	m.Write(0x0000, 0xFF)
	m.Write(0x3FFF, 0xFF)
	if m.Read(0) != 0xC3 || m.Read(0x3FFF) != 0x11 {
		t.Fatal("ROM write was not ignored")
	}
	m.Write(0x4000, 0x22)
	m.Write(0x7FFF, 0x33)
	m.Write(0x8000, 0x44)
	m.Write(0xFFFF, 0x55)
	if m.Read(0x4000) != 0x22 || m.RAM[0] != 0x22 {
		t.Fatal("contended RAM")
	}
	if m.Read(0x7FFF) != 0x33 || m.RAM[0x3FFF] != 0x33 {
		t.Fatal("end of contended RAM")
	}
	if m.Read(0x8000) != 0x44 || m.RAM[0x4000] != 0x44 {
		t.Fatal("uncontended RAM")
	}
	if m.Read(0xFFFF) != 0x55 || m.RAM[0xBFFF] != 0x55 {
		t.Fatal("address wrap is not 16-bit")
	}
	if m.Read(0xC000) != m.RAM[0x8000] {
		t.Fatal("no second page: 0xC000 is the top 16K of the 48K RAM")
	}
}
