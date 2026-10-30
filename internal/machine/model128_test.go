package machine

import "testing"

func new128(t *testing.T) *Machine {
	t.Helper()
	rom := make([]byte, 32768)
	rom[0], rom[1] = 0x10, 0x11
	rom[16384], rom[16385] = 0x20, 0x21
	m := New()
	if err := m.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	if !m.Is128() {
		t.Fatal("32 KB ROM is a 128K")
	}
	return m
}

func TestPagingBankROMAndLock(t *testing.T) {
	m := new128(t)
	if m.MemRead(0) != 0x10 || m.MemRead(1) != 0x11 {
		t.Fatalf("editor ROM %02X %02X", m.MemRead(0), m.MemRead(1))
	}
	m.MemWrite(0, 0xFF)
	if m.MemRead(0) != 0x10 {
		t.Fatal("ROM is not writable")
	}

	m.IOWrite(0x7FFD, 0x10)
	if m.MemRead(0) != 0x20 || m.MemRead(1) != 0x21 {
		t.Fatalf("48K ROM %02X %02X", m.MemRead(0), m.MemRead(1))
	}

	m.IOWrite(0x7FFD, 4)
	m.MemWrite(0xC000, 0xAB)
	m.MemWrite(0x4000, 0xCD)
	m.MemWrite(0x8000, 0xEF)
	m.IOWrite(0x7FFD, 1)
	if m.MemRead(0xC000) != 0 || m.MemRead(0x4000) != 0xCD || m.MemRead(0x8000) != 0xEF {
		t.Fatalf("banks C000=%02X 4000=%02X 8000=%02X", m.MemRead(0xC000), m.MemRead(0x4000), m.MemRead(0x8000))
	}
	m.MemWrite(0xC000, 0x11)
	m.IOWrite(0x7FFD, 4)
	if m.MemRead(0xC000) != 0xAB {
		t.Fatalf("bank 4 %02X", m.MemRead(0xC000))
	}
	m.IOWrite(0x7FFD, 1)
	if m.MemRead(0xC000) != 0x11 {
		t.Fatalf("bank 1 %02X", m.MemRead(0xC000))
	}

	// Bit 5 locks after the write that sets it. Bank 0 is the paged bank.
	m.IOWrite(0x7FFD, 0)
	m.MemWrite(0xC000, 0xAB)
	m.IOWrite(0x7FFD, 0x30)
	if m.MemRead(0) != 0x20 || m.MemRead(0xC000) != 0xAB || m.page != 0x30 {
		t.Fatalf("lock write page %02X rom %02X bank %02X", m.page, m.MemRead(0), m.MemRead(0xC000))
	}
	m.IOWrite(0x7FFD, 0x07)
	if m.page != 0x30 || m.MemRead(0) != 0x20 || m.MemRead(0xC000) != 0xAB {
		t.Fatalf("locked page %02X rom %02X bank %02X", m.page, m.MemRead(0), m.MemRead(0xC000))
	}

	m.Reset()
	if !m.Is128() || m.page != 0 || m.MemRead(0) != 0x10 || m.MemRead(0xC000) != 0 {
		t.Fatal("reset clears paging and keeps the ROM")
	}
}

func TestShadowScreen(t *testing.T) {
	m := new128(t)
	m.IOWrite(0x7FFD, 7)
	m.MemWrite(0xC000, 0x80)
	m.MemWrite(0xD800, 0x07)
	if m.paperAt(0, 0) != 0 {
		t.Fatal("bank 5 is the displayed screen")
	}
	m.IOWrite(0x7FFD, 7|0x08)
	if m.paperAt(0, 0) != 7 {
		t.Fatalf("shadow pixel %d", m.paperAt(0, 0))
	}
	m.IOWrite(0x7FFD, 7)
	if m.paperAt(0, 0) != 0 {
		t.Fatal("screen bit cleared")
	}
}

func TestPageViaCPU(t *testing.T) {
	rom := make([]byte, 32768)
	// LD A,1 / LD BC,$7FFD / OUT (C),A / LD A,$5A / LD ($C000),A / HALT
	copy(rom, []byte{0x3E, 0x01, 0x01, 0xFD, 0x7F, 0xED, 0x79, 0x3E, 0x5A, 0x32, 0x00, 0xC0, 0x76})
	m := New()
	if err := m.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	m.RunFrame()
	if m.MemRead(0xC000) != 0x5A || m.banks[1][0] != 0x5A {
		t.Fatalf("paged write %02X bank %02X", m.MemRead(0xC000), m.banks[1][0])
	}
}

func TestContention128(t *testing.T) {
	m := new128(t)
	if m.ContendMem(0x4000, t128Cont) != 6 {
		t.Fatalf("delay %d", m.ContendMem(0x4000, t128Cont))
	}
	if m.ContendMem(0x4000, t128Cont-1) != 0 || m.ContendMem(0x8000, t128Cont) != 0 {
		t.Fatal("before the window, or bank 2, waits 0")
	}
	if m.ContendMem(0xC000, t128Cont) != 0 {
		t.Fatal("even bank at C000 is not contended")
	}
	m.IOWrite(0x7FFD, 1)
	if m.ContendMem(0xC000, t128Cont) != 6 || m.ContendMem(0xC000, t128Cont+2) != 4 {
		t.Fatalf("odd bank %d %d", m.ContendMem(0xC000, t128Cont), m.ContendMem(0xC000, t128Cont+2))
	}
	if m.ContendMem(0x4000, t128Cont+192*t128Line) != 0 {
		t.Fatal("past the 192 paper lines")
	}
	m.IOWrite(0x7FFD, 0)
	if m.ContendMem(0xC000, t128Cont) != 0 {
		t.Fatal("bank 0 cleared contention")
	}

	idle := New()
	if err := idle.LoadROM(make([]byte, 32768)); err != nil {
		t.Fatal(err)
	}
	idle.RunFrame()
	if idle.FrameT() != t128Frame {
		t.Fatalf("128K frame %d", idle.FrameT())
	}

	m48 := New()
	if err := m48.LoadROM(make([]byte, 16384)); err != nil {
		t.Fatal(err)
	}
	m48.RunFrame()
	if m48.FrameT() != TStatesPerFrame {
		t.Fatalf("48K frame %d", m48.FrameT())
	}
}

func TestSnapshot48On128Locks(t *testing.T) {
	m := new128(t)
	var s Snap
	s.RAM[0] = 0x77
	s.RAM[16384] = 0x66
	s.RAM[32768] = 0x55
	s.PC = 0x1234
	s.Border = 4
	m.ApplySnapshot(s)
	if m.cpu.PC != 0x1234 || m.Border() != 4 {
		t.Fatalf("pc %04X border %d", m.cpu.PC, m.Border())
	}
	if m.MemRead(0x4000) != 0x77 || m.MemRead(0x8000) != 0x66 || m.MemRead(0xC000) != 0x55 {
		t.Fatal("48K RAM did not land in banks 5, 2, and 0")
	}
	if m.MemRead(0) != 0x20 || m.page != 0x30 {
		t.Fatalf("want 48K ROM locked, page %02X rom %02X", m.page, m.MemRead(0))
	}
	m.IOWrite(0x7FFD, 0x07)
	if m.MemRead(0) != 0x20 || m.MemRead(0xC000) != 0x55 {
		t.Fatal("48K snapshot stayed locked")
	}
}

func TestSnapshot128RequiresROM(t *testing.T) {
	m := New()
	if err := m.LoadROM(make([]byte, 16384)); err != nil {
		t.Fatal(err)
	}
	m.MemWrite(0x4000, 0x42)
	var s Snap
	s.Model128 = true
	s.PC = 0x9999
	s.Banks[0][0] = 0x11
	m.ApplySnapshot(s)
	if m.cpu.PC == 0x9999 || m.MemRead(0x4000) != 0x42 || m.ErrorText() == "" {
		t.Fatalf("pc %04X ram %02X err %q", m.cpu.PC, m.MemRead(0x4000), m.ErrorText())
	}
}

func TestSnapshot128BanksAndAY(t *testing.T) {
	m := new128(t)
	var s Snap
	s.Model128 = true
	s.Page = 0x04
	s.Banks[4][0] = 0x5A
	s.Banks[5][0] = 0x81
	s.AYSel = 3
	s.AYReg[3] = 0x44
	s.PC = 0x8000
	m.ApplySnapshot(s)
	if m.cpu.PC != 0x8000 || m.MemRead(0xC000) != 0x5A || m.MemRead(0x4000) != 0x81 {
		t.Fatalf("pc %04X C000 %02X 4000 %02X", m.cpu.PC, m.MemRead(0xC000), m.MemRead(0x4000))
	}
	if m.MemRead(0) != 0x10 || m.IORead(0xFFFD) != 0x44 {
		t.Fatalf("rom %02X ay %02X", m.MemRead(0), m.IORead(0xFFFD))
	}
	m.IOWrite(0x7FFD, 0x10)
	if m.MemRead(0) != 0x20 {
		t.Fatal("unlocked 128 snapshot still pages")
	}
}
