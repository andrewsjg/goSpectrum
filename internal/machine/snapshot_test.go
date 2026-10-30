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

func TestSnapshotRoundTrip(t *testing.T) {
	m := New()
	m.mem.RAM[100] = 0x42
	m.cpu.A, m.cpu.SP, m.cpu.PC, m.cpu.IM = 0x77, 0xFF00, 0x8001, 1
	m.cpu.Halted = true
	m.border = 3
	s := m.Snapshot()
	if s.PC != 0x8000 || s.Model128 {
		t.Fatalf("halted PC %04X", s.PC)
	}
	fresh := New()
	fresh.ApplySnapshot(s)
	if fresh.mem.RAM[100] != 0x42 || fresh.cpu.A != 0x77 || fresh.cpu.SP != 0xFF00 || fresh.border != 3 {
		t.Fatal("48K state not restored")
	}

	m128 := new128(t)
	m128.writePage(0x14)
	m128.banks[4][7] = 0x99
	m128.ay.selectReg(8)
	m128.ay.write(0x0F)
	s = m128.Snapshot()
	if !s.Model128 || s.Page != 0x14 || s.Banks[4][7] != 0x99 || s.AYSel != 8 || s.AYReg[8] != 0x0F {
		t.Fatalf("128K snapshot page %02X sel %d", s.Page, s.AYSel)
	}
	other := new128(t)
	other.ApplySnapshot(s)
	if other.ramPage != 4 || other.banks[4][7] != 0x99 || other.ay.reg[8] != 0x0F {
		t.Fatal("128K state not restored")
	}
}

func TestKempstonPort(t *testing.T) {
	m := New()
	m.SetJoystick(JoyUp | JoyFire)
	if got := m.IORead(0x001F); got != JoyUp|JoyFire {
		t.Fatalf("port 0x1F %02X", got)
	}
	if m.IORead(0x00FF) == JoyUp|JoyFire {
		t.Fatal("A5 high read the joystick")
	}
}

func TestTapeProgress(t *testing.T) {
	var p Player
	if p.Progress() != 0 {
		t.Fatal("empty tape")
	}
	p.Arm([]Pulse{{true, 100}, {false, 300}}, false)
	p.Advance(200)
	if p.Progress() != 0.5 {
		t.Fatalf("half way %v", p.Progress())
	}
	p.Advance(500)
	if p.Progress() != 1 {
		t.Fatal("finished tape")
	}
}
