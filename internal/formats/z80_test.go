package formats

import (
	"encoding/binary"
	"errors"
	"testing"
)

func sampleSnap() Snapshot {
	var s Snapshot
	s.Regs.A = 0x11
	s.Regs.F = 0x22
	s.Regs.B = 0x33
	s.Regs.C = 0x44
	s.Regs.D = 0x55
	s.Regs.E = 0x66
	s.Regs.H = 0x77
	s.Regs.L = 0x88
	s.Regs.Ap = 0x99
	s.Regs.Fp = 0xAA
	s.Regs.Bp = 0xBB
	s.Regs.Cp = 0xCC
	s.Regs.Dp = 0xDD
	s.Regs.Ep = 0xEE
	s.Regs.Hp = 0x01
	s.Regs.Lp = 0x02
	s.Regs.IX = 0x0345
	s.Regs.IY = 0x0678
	s.Regs.SP = 0x8000
	s.Regs.PC = 0x9000
	s.Regs.I = 0x0A
	s.Regs.R = 0x8B
	s.Regs.IFF1 = true
	s.Regs.IFF2 = false
	s.Regs.IM = 2
	s.Border = 5
	s.RAM[0] = 0xFE
	s.RAM[16384] = 0xFD
	s.RAM[32768] = 0xFC
	s.RAM[49151] = 0xFB
	return s
}

func TestZ80V3RoundTrip(t *testing.T) {
	s := sampleSnap()
	raw := WriteZ80(s)
	got, err := ReadZ80(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.PopPC || got.Regs != s.Regs || got.RAM != s.RAM || got.Border != 5 {
		t.Fatalf("round trip pop %v border %d PC %04X", got.PopPC, got.Border, got.Regs.PC)
	}
	if raw[0] != 0x11 || raw[1] != 0x22 {
		t.Fatal("v3 stores A then F")
	}
}

func TestZ80V1Compression(t *testing.T) {
	hdr := make([]byte, 30)
	binary.LittleEndian.PutUint16(hdr[6:], 1)
	hdr[12] = 0x20
	var body []byte
	for i := 0; i < 192; i++ {
		body = append(body, 0xED, 0xED, 0xFF, 0x00)
	}
	body = append(body, 0xED, 0xED, 0xC0, 0x00)
	body = append(body, 0x00, 0xED, 0xED, 0x00)
	got, err := ReadZ80(append(hdr, body...))
	if err != nil {
		t.Fatal(err)
	}
	if got.Regs.PC != 1 || got.RAM != [49152]byte{} || got.PopPC {
		t.Fatal("compressed zeros")
	}

	hdr[12] = 0x20
	body = []byte{0xED, 0x01}
	for i := 0; i < 192; i++ {
		body = append(body, 0xED, 0xED, 0xFF, 0x00)
	}
	body = append(body, 0xED, 0xED, 190, 0x00, 0x00, 0xED, 0xED, 0x00)
	got, err = ReadZ80(append(hdr, body...))
	if err != nil {
		t.Fatal(err)
	}
	if got.RAM[0] != 0xED || got.RAM[1] != 0x01 || got.RAM[2] != 0 {
		t.Fatalf("literal ED %02X %02X", got.RAM[0], got.RAM[1])
	}

	body = []byte{0xED, 0xED, 0x00, 0x11, 0x00, 0xED, 0xED, 0x00}
	if _, err := ReadZ80(append(hdr, body...)); err == nil {
		t.Fatal("count 0")
	}
}

func TestZ80Flags255AndRefusals(t *testing.T) {
	raw := make([]byte, 30+49152)
	binary.LittleEndian.PutUint16(raw[6:], 0x0002)
	raw[12] = 255
	raw[30] = 0x42
	got, err := ReadZ80(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Regs.R != 0x80 || got.RAM[0] != 0x42 || got.Border != 0 {
		t.Fatalf("R %02X ram %02X", got.Regs.R, got.RAM[0])
	}
	raw = append(raw, 0x00)
	if _, err := ReadZ80(raw); err == nil {
		t.Fatal("trailing byte")
	}

	sam := make([]byte, 30)
	binary.LittleEndian.PutUint16(sam[6:], 1)
	sam[12] = 0x10
	if _, err := ReadZ80(sam); !errors.Is(err, ErrBeyond48K) {
		t.Fatal(err)
	}
	im := make([]byte, 30+49152)
	binary.LittleEndian.PutUint16(im[6:], 1)
	im[29] = 3
	if _, err := ReadZ80(im); err == nil || errors.Is(err, ErrBeyond48K) {
		t.Fatal(err)
	}

	plus3 := make([]byte, 86)
	binary.LittleEndian.PutUint16(plus3[30:], 54)
	plus3[34] = 7
	if _, err := ReadZ80(plus3); !errors.Is(err, ErrBeyond48K) {
		t.Fatal(err)
	}
	missing := make([]byte, 86)
	binary.LittleEndian.PutUint16(missing[30:], 54)
	if _, err := ReadZ80(missing); err == nil || errors.Is(err, ErrBeyond48K) {
		t.Fatal(err)
	}
}

func TestZ80PageOrderAndV2(t *testing.T) {
	// Pages stored as 5, 8, 4 must still land on C000, 4000, 8000.
	var hdr [55]byte
	binary.LittleEndian.PutUint16(hdr[30:], 23)
	binary.LittleEndian.PutUint16(hdr[32:], 0x4321)
	hdr[34] = 0
	var raw []byte
	raw = append(raw, hdr[:]...)
	page := func(id byte, fill byte) {
		var n [2]byte
		binary.LittleEndian.PutUint16(n[:], 0xFFFF)
		raw = append(raw, n[:]...)
		raw = append(raw, id)
		buf := make([]byte, 16384)
		buf[0] = fill
		raw = append(raw, buf...)
	}
	page(5, 0x05)
	page(8, 0x08)
	page(4, 0x04)
	got, err := ReadZ80(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Regs.PC != 0x4321 || got.RAM[0] != 0x08 || got.RAM[16384] != 0x04 || got.RAM[32768] != 0x05 || got.PopPC {
		t.Fatalf("pages %02X %02X %02X PC %04X", got.RAM[0], got.RAM[16384], got.RAM[32768], got.Regs.PC)
	}

	// Compressed page 8 of zeros, raw 4 and 5.
	comp := []byte{}
	for i := 0; i < 64; i++ {
		comp = append(comp, 0xED, 0xED, 0xFF, 0x00)
	}
	comp = append(comp, 0xED, 0xED, 64, 0x00)
	var file []byte
	file = append(file, hdr[:]...)
	var n [2]byte
	binary.LittleEndian.PutUint16(n[:], uint16(len(comp)))
	file = append(file, n[:]...)
	file = append(file, 8)
	file = append(file, comp...)
	pageOnto := func(id, fill byte) {
		binary.LittleEndian.PutUint16(n[:], 0xFFFF)
		file = append(file, n[:]...)
		file = append(file, id)
		buf := make([]byte, 16384)
		buf[0] = fill
		file = append(file, buf...)
	}
	pageOnto(4, 0x44)
	pageOnto(5, 0x55)
	got, err = ReadZ80(file)
	if err != nil {
		t.Fatal(err)
	}
	if got.RAM[0] != 0 || got.RAM[16384] != 0x44 || got.RAM[32768] != 0x55 {
		t.Fatal("compressed page")
	}

	other := append([]byte{}, hdr[:]...)
	binary.LittleEndian.PutUint16(n[:], 0xFFFF)
	other = append(other, n[:]...)
	other = append(other, 3)
	other = append(other, make([]byte, 16384)...)
	if _, err := ReadZ80(other); !errors.Is(err, ErrBeyond48K) {
		t.Fatal(err)
	}
}

func TestZ80128(t *testing.T) {
	var hdr [55]byte
	binary.LittleEndian.PutUint16(hdr[30:], 23)
	binary.LittleEndian.PutUint16(hdr[32:], 0x8000)
	hdr[34] = 3
	hdr[35] = 0x03
	hdr[38] = 2
	hdr[41] = 0x77
	var raw []byte
	raw = append(raw, hdr[:]...)
	for page := byte(3); page <= 10; page++ {
		var n [2]byte
		binary.LittleEndian.PutUint16(n[:], 0xFFFF)
		raw = append(raw, n[:]...)
		raw = append(raw, page)
		buf := make([]byte, 16384)
		buf[0] = page
		raw = append(raw, buf...)
	}
	got, err := ReadZ80(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Machine128 || got.PopPC || got.Regs.PC != 0x8000 || got.Page != 0x03 || got.AYSel != 2 || got.AYReg[2] != 0x77 {
		t.Fatalf("PC %04X page %02X ay %02X/%02X", got.Regs.PC, got.Page, got.AYSel, got.AYReg[2])
	}
	for page := byte(3); page <= 10; page++ {
		if got.Banks[page-3][0] != page {
			t.Fatalf("page %d %02X", page, got.Banks[page-3][0])
		}
	}
	if got.RAM[0] != 8 || got.RAM[16384] != 5 || got.RAM[32768] != 6 {
		t.Fatalf("view %02X %02X %02X", got.RAM[0], got.RAM[16384], got.RAM[32768])
	}

	hdr[34] = 4
	if _, err := ReadZ80(append([]byte{}, hdr[:]...)); !errors.Is(err, ErrBeyond48K) {
		t.Fatal(err)
	}

	var v3 [86]byte
	binary.LittleEndian.PutUint16(v3[30:], 54)
	binary.LittleEndian.PutUint16(v3[32:], 0x4000)
	v3[34] = 4
	v3[35] = 0x00
	missing := append([]byte{}, v3[:]...)
	for page := byte(3); page <= 9; page++ {
		var n [2]byte
		binary.LittleEndian.PutUint16(n[:], 0xFFFF)
		missing = append(missing, n[:]...)
		missing = append(missing, page)
		missing = append(missing, make([]byte, 16384)...)
	}
	if _, err := ReadZ80(missing); err == nil || errors.Is(err, ErrBeyond48K) {
		t.Fatal(err)
	}
}
