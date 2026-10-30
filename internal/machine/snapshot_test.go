package machine

import "testing"

func TestApplySnapshotRetnOnce(t *testing.T) {
	m := New()
	m.mem.ROM[0] = 0x99
	m.beep.Speaker = true
	m.frameT = 1234
	m.mem.RAM[0] = 0x11

	var s Snap
	s.A = 0xAB
	s.F = 0x01
	s.B = 0x02
	s.Ap = 0x10
	s.Fp = 0x20
	s.IX = 0x1111
	s.IY = 0x2222
	s.I = 0x3C
	s.R = 0x81
	s.IFF2 = true
	s.IM = 2
	s.Border = 0x15
	s.SP = 0x8000
	s.PopPC = true
	s.RAM[0x4000] = 0x34
	s.RAM[0x4001] = 0x12
	s.RAM[0x4002] = 0x78
	s.RAM[0x4003] = 0x56
	s.RAM[0] = 0x5A
	m.ApplySnapshot(s)
	if m.cpu.PC != 0x1234 {
		t.Fatalf("PC %04X", m.cpu.PC)
	}
	if m.cpu.SP != 0x8002 {
		t.Fatalf("SP %04X", m.cpu.SP)
	}
	if m.cpu.PC == 0x5678 {
		t.Fatal("second pop")
	}
	if !m.cpu.IFF1 || !m.cpu.IFF2 {
		t.Fatal("IFF")
	}
	if m.cpu.A != 0xAB || m.cpu.Ap != 0x10 || m.cpu.IX != 0x1111 || m.cpu.R != 0x81 || m.cpu.IM != 2 {
		t.Fatal("registers")
	}
	if m.border != 5 || m.Speaker() || m.FrameT() != 0 {
		t.Fatalf("border %d speaker %v t %d", m.border, m.Speaker(), m.FrameT())
	}
	if m.mem.RAM[0] != 0x5A {
		t.Fatal("RAM not replaced")
	}

	var wrap Snap
	wrap.PopPC = true
	wrap.SP = 0xFFFF
	wrap.IFF2 = true
	wrap.RAM[0xBFFF] = 0x34
	m.mem.ROM[0] = 0x12
	m.ApplySnapshot(wrap)
	if m.cpu.PC != 0x1234 || m.cpu.SP != 0x0001 || !m.cpu.IFF1 {
		t.Fatalf("wrap PC %04X SP %04X", m.cpu.PC, m.cpu.SP)
	}
}

func TestRefusedSnapshotLeavesState(t *testing.T) {
	m := New()
	m.mem.RAM[0] = 0x11
	m.cpu.A = 0x22
	m.cpu.PC = 0x0000
	m.SetError("the file targets a machine configuration beyond the emulated 48K")
	if m.mem.RAM[0] != 0x11 || m.cpu.A != 0x22 || m.cpu.PC != 0 {
		t.Fatal("refused snapshot changed the machine")
	}
	if m.Status() != "error" {
		t.Fatal(m.Status())
	}
}
