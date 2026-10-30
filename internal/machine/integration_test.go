package machine

import (
	"errors"
	"os"
	"testing"

	"go.spectrum/internal/formats"
)

func TestIntegrationTapeAndSnapshots(t *testing.T) {
	rom, err := os.ReadFile("../../testdata/test.rom")
	if err != nil {
		t.Fatal(err)
	}

	block := []byte{0xFF, 0x00}
	sum := byte(0)
	for _, b := range block {
		sum ^= b
	}
	block = append(block, sum)
	_, tape, err := formats.ReadTAP(formats.WriteTAP([][]byte{block}))
	if err != nil {
		t.Fatal(err)
	}
	m := New()
	if err := m.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	m.ArmTape(pulses(tape.Edges), tape.ForceLow, false)
	m.tapeHold = false // this test checks that edges reach the port
	if m.IORead(0xFE)&0x40 == 0 {
		t.Fatal("first tape edge is not high on port 0xFE bit 6")
	}
	m.Advance(tape.Edges[0].Dur)
	if m.IORead(0xFE)&0x40 != 0 {
		t.Fatal("edge did not flip")
	}

	var snap formats.Snapshot
	snap.Regs.SP = 0x8000
	snap.Regs.PC = 0x4000
	snap.Regs.A = 0x42
	snap.RAM[0] = 0x5A
	snap.RAM[49151] = 0xA5
	snap.Border = 3
	raw, err := formats.WriteSNA(snap)
	if err != nil || len(raw) != 49179 {
		t.Fatal(err, len(raw))
	}
	got, err := formats.ReadSNA(raw)
	if err != nil {
		t.Fatal(err)
	}
	m = New()
	if err := m.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	m.ApplySnapshot(toMachine(got))
	if m.mem.RAM != got.RAM {
		t.Fatal("SNA RAM mismatch")
	}
	if m.cpu.PC != 0x4000 || m.cpu.SP != 0x8000 || m.cpu.A != 0x42 || m.border != 3 || m.frameT != 0 {
		t.Fatalf("PC %04X SP %04X A %02X border %d t %d", m.cpu.PC, m.cpu.SP, m.cpu.A, m.border, m.frameT)
	}

	snap.PopPC = false
	zraw := formats.WriteZ80(snap)
	zgot, err := formats.ReadZ80(zraw)
	if err != nil {
		t.Fatal(err)
	}
	if zgot.PopPC {
		t.Fatal("Z80 popPC")
	}
	m = New()
	if err := m.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	m.mem.RAM[0] = 0x11
	m.ApplySnapshot(toMachine(zgot))
	if m.mem.RAM != zgot.RAM || m.cpu.PC != snap.Regs.PC || m.cpu.A != 0x42 {
		t.Fatal("Z80 state")
	}

	plus3 := make([]byte, 86)
	plus3[30] = 54
	plus3[34] = 7
	if _, err := formats.ReadZ80(plus3); !errors.Is(err, formats.ErrBeyond48K) {
		t.Fatal(err)
	}
	m = New()
	if err := m.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	m.mem.RAM[0] = 0xAB
	m.cpu.A = 0x22
	m.SetError(formats.ErrBeyond48K.Error())
	if m.mem.RAM[0] != 0xAB || m.cpu.A != 0x22 || m.Status() != "error" {
		t.Fatal("refused Z80 changed the machine")
	}
}

func pulses(in []formats.Pulse) []Pulse {
	out := make([]Pulse, len(in))
	for i, p := range in {
		out[i] = Pulse{Level: p.Level, Dur: p.Dur}
	}
	return out
}

func toMachine(s formats.Snapshot) Snap {
	r := s.Regs
	return Snap{
		A: r.A, F: r.F, B: r.B, C: r.C, D: r.D, E: r.E, H: r.H, L: r.L,
		Ap: r.Ap, Fp: r.Fp, Bp: r.Bp, Cp: r.Cp, Dp: r.Dp, Ep: r.Ep, Hp: r.Hp, Lp: r.Lp,
		IX: r.IX, IY: r.IY, SP: r.SP, PC: r.PC, I: r.I, R: r.R,
		IFF1: r.IFF1, IFF2: r.IFF2, IM: r.IM,
		Border: s.Border, RAM: s.RAM, PopPC: s.PopPC,
		Model128: s.Machine128, Banks: s.Banks, Page: s.Page,
		AYSel: s.AYSel, AYReg: s.AYReg,
	}
}
