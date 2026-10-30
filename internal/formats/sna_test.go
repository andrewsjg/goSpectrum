package formats

import (
	"errors"
	"testing"
)

func TestSNARoundTrip(t *testing.T) {
	var s Snapshot
	s.Regs.I = 0x3C
	s.Regs.A = 0x12
	s.Regs.F = 0x34
	s.Regs.B = 0x56
	s.Regs.C = 0x78
	s.Regs.D = 0x9A
	s.Regs.E = 0xBC
	s.Regs.H = 0xDE
	s.Regs.L = 0xF0
	s.Regs.Ap = 0x11
	s.Regs.Fp = 0x22
	s.Regs.Bp = 0x33
	s.Regs.Cp = 0x44
	s.Regs.Dp = 0x55
	s.Regs.Ep = 0x66
	s.Regs.Hp = 0x77
	s.Regs.Lp = 0x88
	s.Regs.IX = 0x1234
	s.Regs.IY = 0x5678
	s.Regs.SP = 0x8000
	s.Regs.PC = 0x4000
	s.Regs.R = 0x81
	s.Regs.IFF2 = true
	s.Regs.IM = 2
	s.Border = 5
	s.RAM[0] = 0xAB
	raw, err := WriteSNA(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 49179 {
		t.Fatalf("len %d", len(raw))
	}
	if raw[21] != 0x34 || raw[22] != 0x12 {
		t.Fatal("AF is F then A")
	}
	if raw[19]&0x04 == 0 {
		t.Fatal("IFF2 bit")
	}
	got, err := ReadSNA(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !got.PopPC || got.Regs.PC != 0 || got.Regs.SP != 0x7FFE {
		t.Fatalf("pop %v PC %04X SP %04X", got.PopPC, got.Regs.PC, got.Regs.SP)
	}
	if got.RAM[0x3FFE] != 0x00 || got.RAM[0x3FFF] != 0x40 {
		t.Fatalf("pushed PC %02X %02X", got.RAM[0x3FFE], got.RAM[0x3FFF])
	}
	if got.RAM[0] != 0xAB || got.Regs.A != 0x12 || got.Regs.F != 0x34 || !got.Regs.IFF2 || got.Border != 5 || got.Regs.IM != 2 || got.Regs.R != 0x81 {
		t.Fatalf("fields %+v border %d", got.Regs, got.Border)
	}
	if got.Regs.IX != 0x1234 || got.Regs.IY != 0x5678 || got.Regs.Lp != 0x88 || got.Regs.Ap != 0x11 {
		t.Fatal("alternate or index")
	}
}

func TestSNA128(t *testing.T) {
	b := make([]byte, 131103)
	b[21] = 0x01
	b[22] = 0x42
	b[26] = 2
	b[49179] = 0x34
	b[49180] = 0x12
	b[49181] = 0x04
	b[27] = 0x51
	b[27+16384] = 0x21
	b[27+32768] = 0x41
	rest := []int{0, 1, 3, 6, 7}
	for i, bank := range rest {
		b[49183+i*16384] = byte(0xA0 + bank)
	}
	got, err := ReadSNA(b)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Machine128 || got.PopPC || got.Regs.PC != 0x1234 || got.Page != 0x04 || got.Regs.A != 0x42 || got.Border != 2 {
		t.Fatalf("header pop %v PC %04X page %02X", got.PopPC, got.Regs.PC, got.Page)
	}
	if got.Banks[5][0] != 0x51 || got.Banks[2][0] != 0x21 || got.Banks[4][0] != 0x41 {
		t.Fatal("paged banks")
	}
	for _, bank := range rest {
		if got.Banks[bank][0] != byte(0xA0+bank) {
			t.Fatalf("bank %d %02X", bank, got.Banks[bank][0])
		}
	}
	if got.RAM[0] != 0x51 || got.RAM[16384] != 0x21 || got.RAM[32768] != 0x41 {
		t.Fatal("48K view")
	}

	b[49182] = 1
	if _, err := ReadSNA(b); err == nil || errors.Is(err, ErrBeyond48K) {
		t.Fatal(err)
	}
	b[49182] = 0
	b[49181] = 5
	if _, err := ReadSNA(b); err == nil || errors.Is(err, ErrBeyond48K) {
		t.Fatal(err)
	}
	if _, err := ReadSNA(make([]byte, 147487)); !errors.Is(err, ErrBeyond48K) {
		t.Fatal(err)
	}
}

func TestSNALength(t *testing.T) {
	_, err := ReadSNA(make([]byte, 49180))
	if !errors.Is(err, ErrBeyond48K) {
		t.Fatal(err)
	}
	_, err = ReadSNA(make([]byte, 100))
	if err == nil || errors.Is(err, ErrBeyond48K) {
		t.Fatal(err)
	}
	badIM := make([]byte, 49179)
	badIM[25] = 3
	if _, err := ReadSNA(badIM); err == nil || errors.Is(err, ErrBeyond48K) {
		t.Fatal(err)
	}
}
