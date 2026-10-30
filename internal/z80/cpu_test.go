package z80

import "testing"

type ramBus struct {
	m          [65536]byte
	t          int
	irq        bool
	contendMem func(uint16, int) int
	contendIO  func(uint16, int) int
	lastOut    uint16
	lastOutV   byte
	outs       int
}

func (b *ramBus) T() int                    { return b.t }
func (b *ramBus) Advance(n int)             { b.t += n }
func (b *ramBus) MemRead(a uint16) byte     { return b.m[a] }
func (b *ramBus) MemWrite(a uint16, v byte) { b.m[a] = v }
func (b *ramBus) IORead(uint16) byte        { return 0xFF }
func (b *ramBus) IOWrite(p uint16, v byte) {
	b.lastOut, b.lastOutV = p, v
	b.outs++
}
func (b *ramBus) ContendMem(a uint16, t int) int {
	if b.contendMem != nil {
		return b.contendMem(a, t)
	}
	return 0
}
func (b *ramBus) ContendIO(p uint16, t int) int {
	if b.contendIO != nil {
		return b.contendIO(p, t)
	}
	return 0
}
func (b *ramBus) InterruptAsserted() bool { return b.irq }
func (b *ramBus) InterruptAck() byte      { return 0xFF }

func (b *ramBus) run(code ...byte) int {
	copy(b.m[:], code)
	var c CPU
	c.Reset()
	return c.Step(b)
}

func TestReset(t *testing.T) {
	var c CPU
	c.A, c.F, c.B, c.PC, c.I, c.R = 1, 2, 3, 4, 5, 6
	c.IFF1, c.IFF2, c.IM, c.Halted = true, true, 2, true
	c.IX, c.IY, c.SP = 9, 8, 7
	c.Reset()
	if c.PC != 0 || c.I != 0 || c.R != 0 || c.IFF1 || c.IFF2 || c.IM != 0 || c.Halted {
		t.Fatalf("control state %+v", c)
	}
	if c.SP != 0xFFFF {
		t.Fatalf("SP %04X", c.SP)
	}
	if c.A|c.F|c.B|c.C|c.D|c.E|c.H|c.L != 0 || c.IX != 0 || c.IY != 0 {
		t.Fatalf("registers not cleared")
	}
}

func TestRBit7Preserved(t *testing.T) {
	var b ramBus
	var c CPU
	c.Reset()
	c.R = 0x7F
	b.m[0] = 0x00
	c.Step(&b)
	if c.R != 0x00 {
		t.Fatalf("wrap R=%02X", c.R)
	}
	c.PC = 0
	c.R = 0xFF
	c.Step(&b)
	if c.R != 0x80 {
		t.Fatalf("bit7 R=%02X", c.R)
	}
	// Two M1 cycles for a prefix.
	c.PC = 0
	c.R = 0x00
	b.m[0], b.m[1] = 0xDD, 0x00
	c.Step(&b)
	if c.R != 0x02 {
		t.Fatalf("prefix R=%02X", c.R)
	}
}

func TestBaseTimings(t *testing.T) {
	cases := []struct {
		name string
		code []byte
		want int
	}{
		{"NOP", []byte{0x00}, 4},
		{"LD r,r", []byte{0x78}, 4},
		{"LD r,n", []byte{0x3E, 0x01}, 7},
		{"LD r,(HL)", []byte{0x7E}, 7},
		{"LD (HL),r", []byte{0x77}, 7},
		{"LD (HL),n", []byte{0x36, 0x00}, 10},
		{"LD A,(BC)", []byte{0x0A}, 7},
		{"LD (BC),A", []byte{0x02}, 7},
		{"LD A,(nn)", []byte{0x3A, 0x00, 0x00}, 13},
		{"LD (nn),A", []byte{0x32, 0x00, 0x00}, 13},
		{"LD dd,nn", []byte{0x01, 0x00, 0x00}, 10},
		{"LD IX,nn", []byte{0xDD, 0x21, 0x34, 0x12}, 14},
		{"LD HL,(nn)", []byte{0x2A, 0x00, 0x80}, 16},
		{"LD (nn),HL", []byte{0x22, 0x00, 0x80}, 16},
		{"LD SP,HL", []byte{0xF9}, 6},
		{"LD SP,IX", []byte{0xDD, 0xF9}, 10},
		{"PUSH", []byte{0xC5}, 11},
		{"PUSH IX", []byte{0xDD, 0xE5}, 15},
		{"POP", []byte{0xC1}, 10},
		{"POP IX", []byte{0xDD, 0xE1}, 14},
		{"EX DE,HL", []byte{0xEB}, 4},
		{"EX AF", []byte{0x08}, 4},
		{"EXX", []byte{0xD9}, 4},
		{"EX (SP),HL", []byte{0xE3}, 19},
		{"EX (SP),IX", []byte{0xDD, 0xE3}, 23},
		{"ALU r", []byte{0x80}, 4},
		{"ALU n", []byte{0xC6, 0x01}, 7},
		{"ALU (HL)", []byte{0x86}, 7},
		{"ALU (IX+d)", []byte{0xDD, 0x86, 0x00}, 19},
		{"INC r", []byte{0x3C}, 4},
		{"INC (HL)", []byte{0x34}, 11},
		{"INC (IX+d)", []byte{0xDD, 0x34, 0x00}, 23},
		{"INC ss", []byte{0x03}, 6},
		{"INC IX", []byte{0xDD, 0x23}, 10},
		{"ADD HL", []byte{0x09}, 11},
		{"ADD IX", []byte{0xDD, 0x09}, 15},
		{"ADC HL", []byte{0xED, 0x4A}, 15},
		{"DAA", []byte{0x27}, 4},
		{"DJNZ taken", []byte{0x06, 0x02, 0x10, 0xFE}, 0}, // checked below
		{"JR", []byte{0x18, 0x00}, 12},
		{"JR NZ not", []byte{0x28, 0x00}, 7}, // Z is clear after reset, so JR Z is not taken... wait JR Z opcode is 28. Z clear, not taken = 7. Good.
		{"JP", []byte{0xC3, 0x00, 0x00}, 10},
		{"JP (HL)", []byte{0xE9}, 4},
		{"JP (IX)", []byte{0xDD, 0xE9}, 8},
		{"CALL", []byte{0xCD, 0x00, 0x00}, 17},
		{"RET", []byte{0xC9}, 10},
		{"RST", []byte{0xC7}, 11},
		{"HALT", []byte{0x76}, 4},
		{"DI", []byte{0xF3}, 4},
		{"IN A,(n)", []byte{0xDB, 0xFE}, 11},
		{"OUT (n),A", []byte{0xD3, 0xFE}, 11},
		{"IN r,(C)", []byte{0xED, 0x78}, 12},
		{"CB r", []byte{0xCB, 0x00}, 8},
		{"CB (HL)", []byte{0xCB, 0x06}, 15},
		{"BIT (HL)", []byte{0xCB, 0x46}, 12},
		{"BIT (IX+d)", []byte{0xDD, 0xCB, 0x00, 0x46}, 20},
		{"CB (IX+d)", []byte{0xDD, 0xCB, 0x00, 0x06}, 23},
		{"LDI", []byte{0xED, 0xA0}, 16},
		{"NEG", []byte{0xED, 0x44}, 8},
		{"IM 1", []byte{0xED, 0x56}, 8},
		{"LD A,I", []byte{0xED, 0x57}, 9},
		{"RLD", []byte{0xED, 0x6F}, 18},
	}
	for _, tc := range cases {
		if tc.want == 0 {
			continue
		}
		var b ramBus
		var c CPU
		c.Reset()
		copy(b.m[:], tc.code)
		got := c.Step(&b)
		if got != tc.want {
			t.Errorf("%s: %d T, want %d", tc.name, got, tc.want)
		}
	}
	// DJNZ taken is 13, not taken 8.
	var b ramBus
	var c CPU
	c.Reset()
	c.B = 2
	b.m[0], b.m[1] = 0x10, 0x00
	if got := c.Step(&b); got != 13 {
		t.Errorf("DJNZ taken %d", got)
	}
	c.PC = 0
	c.B = 1
	b.t = 0
	if got := c.Step(&b); got != 8 {
		t.Errorf("DJNZ not %d", got)
	}
	// RET cc / JR cc / CALL cc / LDIR repeat.
	c = CPU{}
	c.Reset()
	b = ramBus{}
	b.m[0], b.m[1] = 0xC0, 0x00 // RET NZ, Z clear, taken
	if got := c.Step(&b); got != 11 {
		t.Errorf("RET cc taken %d", got)
	}
	c.PC = 0
	c.F = FlagZ
	b.t = 0
	b.m[0] = 0xC0 // RET NZ not taken
	if got := c.Step(&b); got != 5 {
		t.Errorf("RET cc not %d", got)
	}
	c = CPU{}
	c.Reset()
	b = ramBus{}
	c.SetBC(0)
	b.m[0], b.m[1] = 0xED, 0xB0 // LDIR with BC 0 after one: we set BC=1 so it copies once and stops?
	c.SetBC(1)
	if got := c.Step(&b); got != 16 {
		t.Errorf("LDIR stop %d", got)
	}
	c.PC = 0
	c.SetBC(2)
	b.t = 0
	if got := c.Step(&b); got != 21 {
		t.Errorf("LDIR repeat %d", got)
	}
}

func TestALUFlags(t *testing.T) {
	var b ramBus
	var c CPU
	c.Reset()
	c.A = 0x0F
	b.m[0], b.m[1] = 0xC6, 0x01 // ADD A,1
	c.Step(&b)
	if c.A != 0x10 || c.F&FlagH == 0 || c.F&FlagN != 0 || c.F&FlagC != 0 {
		t.Fatalf("ADD A=%02X F=%02X", c.A, c.F)
	}
	c.PC = 0
	c.A = 0x7F
	b.t = 0
	b.m[1] = 0x01
	c.Step(&b)
	if c.A != 0x80 || c.F&FlagP == 0 || c.F&FlagS == 0 {
		t.Fatalf("overflow A=%02X F=%02X", c.A, c.F)
	}
	c.PC = 0
	c.A = 0xFF
	b.t = 0
	c.Step(&b)
	if c.A != 0x00 || c.F&FlagC == 0 || c.F&FlagZ == 0 {
		t.Fatalf("carry A=%02X F=%02X", c.A, c.F)
	}
	// CP takes X/Y from the operand.
	c = CPU{}
	c.Reset()
	b = ramBus{}
	c.A = 0x00
	b.m[0], b.m[1] = 0xFE, 0x28 // CP 0x28, bits 3 and 5 set
	c.Step(&b)
	if c.A != 0x00 {
		t.Fatal("CP wrote A")
	}
	if c.F&FlagX == 0 || c.F&FlagY == 0 || c.F&FlagN == 0 {
		t.Fatalf("CP XY F=%02X", c.F)
	}
	// AND sets H and clears C, N.
	c = CPU{}
	c.Reset()
	b = ramBus{}
	c.A = 0xFF
	c.F = FlagC | FlagN
	b.m[0], b.m[1] = 0xE6, 0x0F
	c.Step(&b)
	if c.A != 0x0F || c.F&FlagH == 0 || c.F&FlagC != 0 || c.F&FlagN != 0 {
		t.Fatalf("AND A=%02X F=%02X", c.A, c.F)
	}
	// INC preserves C and sets PV from 0x7F.
	c = CPU{}
	c.Reset()
	b = ramBus{}
	c.A = 0x7F
	c.F = FlagC
	b.m[0] = 0x3C
	c.Step(&b)
	if c.A != 0x80 || c.F&FlagC == 0 || c.F&FlagP == 0 || c.F&FlagN != 0 {
		t.Fatalf("INC A=%02X F=%02X", c.A, c.F)
	}
	// DEC sets N and PV from 0x80.
	c.PC = 0
	c.A = 0x80
	b.t = 0
	b.m[0] = 0x3D
	c.Step(&b)
	if c.A != 0x7F || c.F&FlagN == 0 || c.F&FlagP == 0 || c.F&FlagC == 0 {
		t.Fatalf("DEC A=%02X F=%02X", c.A, c.F)
	}
}

func TestSCFAndBIT(t *testing.T) {
	var b ramBus
	var c CPU
	c.Reset()
	c.A = 0x28
	c.F = FlagS | FlagZ | FlagP | FlagH | FlagN
	b.m[0] = 0x37
	c.Step(&b)
	if c.F&FlagC == 0 || c.F&FlagH != 0 || c.F&FlagN != 0 {
		t.Fatalf("SCF F=%02X", c.F)
	}
	if c.F&(FlagS|FlagZ|FlagP) != FlagS|FlagZ|FlagP {
		t.Fatalf("SCF preserved F=%02X", c.F)
	}
	if c.F&(FlagX|FlagY) != FlagX|FlagY {
		t.Fatalf("SCF XY F=%02X", c.F)
	}
	c = CPU{}
	c.Reset()
	b = ramBus{}
	c.A = 0x80
	b.m[0], b.m[1] = 0xCB, 0x7F // BIT 7,A
	c.Step(&b)
	if c.F&FlagZ != 0 || c.F&FlagS == 0 || c.F&FlagH == 0 || c.F&FlagN != 0 || c.F&FlagP != 0 {
		t.Fatalf("BIT 7 F=%02X", c.F)
	}
	c = CPU{}
	c.Reset()
	b = ramBus{}
	c.A = 0x00
	b.m[0], b.m[1] = 0xCB, 0x47 // BIT 0,A
	c.Step(&b)
	if c.F&(FlagZ|FlagP|FlagH) != FlagZ|FlagP|FlagH || c.F&FlagS != 0 || c.F&FlagN != 0 {
		t.Fatalf("BIT 0 F=%02X", c.F)
	}
}

func TestSLL(t *testing.T) {
	var b ramBus
	var c CPU
	c.Reset()
	c.A = 0x80
	b.m[0], b.m[1] = 0xCB, 0x37 // SLL A
	c.Step(&b)
	if c.A != 0x01 || c.F&FlagC == 0 {
		t.Fatalf("SLL A=%02X F=%02X", c.A, c.F)
	}
}

func TestMEMPTR(t *testing.T) {
	var b ramBus
	var c CPU
	c.Reset()
	c.SetBC(0x1234)
	b.m[0] = 0x0A
	b.m[0x1234] = 0x99
	c.Step(&b)
	if c.A != 0x99 || c.MEMPTR != 0x1235 {
		t.Fatalf("LD A,(BC) A=%02X MEMPTR=%04X", c.A, c.MEMPTR)
	}
	c = CPU{}
	c.Reset()
	b = ramBus{}
	c.A = 0xAB
	c.SetBC(0x1234)
	b.m[0] = 0x02
	c.Step(&b)
	if b.m[0x1234] != 0xAB || c.MEMPTR != 0xAB35 {
		t.Fatalf("LD (BC),A mem=%02X MEMPTR=%04X", b.m[0x1234], c.MEMPTR)
	}
	c = CPU{}
	c.Reset()
	b = ramBus{}
	c.SetHL(0x1000)
	c.SetBC(0x0001)
	b.m[0] = 0x09
	c.Step(&b)
	if c.HL() != 0x1001 || c.MEMPTR != 0x1001 {
		t.Fatalf("ADD HL HL=%04X MEMPTR=%04X", c.HL(), c.MEMPTR)
	}
	c = CPU{}
	c.Reset()
	b = ramBus{}
	c.A = 0x10
	b.m[0], b.m[1] = 0xDB, 0xFE
	c.Step(&b)
	if c.MEMPTR != 0x10FF {
		t.Fatalf("IN MEMPTR %04X", c.MEMPTR)
	}
	c = CPU{}
	c.Reset()
	b = ramBus{}
	c.A = 0xAB
	b.m[0], b.m[1] = 0xD3, 0xFE
	c.Step(&b)
	if c.MEMPTR != 0xABFF {
		t.Fatalf("OUT MEMPTR %04X", c.MEMPTR)
	}
}

func TestLDAI(t *testing.T) {
	var b ramBus
	var c CPU
	c.Reset()
	c.I = 0x42
	c.IFF2 = true
	b.m[0], b.m[1] = 0xED, 0x57
	got := c.Step(&b)
	if got != 9 || c.A != 0x42 || c.F&FlagP == 0 || c.F&FlagH != 0 || c.F&FlagN != 0 {
		t.Fatalf("LD A,I A=%02X F=%02X t=%d", c.A, c.F, got)
	}
}

func TestIndexPrefix(t *testing.T) {
	var b ramBus
	var c CPU
	c.Reset()
	// Last prefix wins: FD then DD then LD IX,nn.
	b.m[0], b.m[1], b.m[2], b.m[3], b.m[4], b.m[5] = 0xFD, 0xDD, 0x21, 0x34, 0x12, 0x00
	c.Step(&b)
	if c.IX != 0x1234 || c.IY != 0 {
		t.Fatalf("IX=%04X IY=%04X", c.IX, c.IY)
	}
	// Prefix on an instruction that does not use HL is only time and R.
	c = CPU{}
	c.Reset()
	b = ramBus{}
	c.SetHL(0x1111)
	b.m[0], b.m[1] = 0xDD, 0xEB // EX DE,HL, prefix ignored
	c.Step(&b)
	if c.HL() != 0x0000 || c.DE() != 0x1111 {
		t.Fatalf("HL=%04X DE=%04X", c.HL(), c.DE())
	}
	// LD L,(IY+d) writes real L. IY itself stays put.
	c = CPU{}
	c.Reset()
	b = ramBus{}
	c.IY = 0x5C3A
	c.L = 0x11
	b.m[0], b.m[1], b.m[2] = 0xFD, 0x6E, 0x0E
	b.m[0x5C48] = 0x38
	c.Step(&b)
	if c.L != 0x38 || c.IY != 0x5C3A {
		t.Fatalf("LD L,(IY+d) L=%02X IY=%04X", c.L, c.IY)
	}
	// LD (IY+d),H stores real H.
	c.PC = 0
	c.H = 0xAB
	b.m[0], b.m[1], b.m[2] = 0xFD, 0x74, 0x0E
	c.Step(&b)
	if b.m[0x5C48] != 0xAB || c.IY != 0x5C3A || c.H != 0xAB {
		t.Fatalf("LD (IY+d),H mem=%02X IY=%04X H=%02X", b.m[0x5C48], c.IY, c.H)
	}
}

func TestIM0AndIM1(t *testing.T) {
	for _, im := range []byte{0, 1} {
		var b ramBus
		var c CPU
		c.Reset()
		c.IFF1 = true
		c.IM = im
		c.PC = 0x0100
		c.SP = 0x8000
		b.irq = true
		n := c.Step(&b)
		if n != 13 || c.PC != 0x0038 {
			t.Fatalf("IM %d PC=%04X t=%d", im, c.PC, n)
		}
		if c.IFF1 || c.IFF2 {
			t.Fatalf("IM %d IFF not cleared", im)
		}
		if b.m[0x7FFF] != 0x01 || b.m[0x7FFE] != 0x00 || c.SP != 0x7FFE {
			t.Fatalf("IM %d stack SP=%04X %02X %02X", im, c.SP, b.m[0x7FFE], b.m[0x7FFF])
		}
	}
}

func TestIM2(t *testing.T) {
	var b ramBus
	var c CPU
	c.Reset()
	c.IFF1, c.IFF2 = true, true
	c.IM = 2
	c.I = 0xFD
	c.PC = 0x0100
	c.SP = 0x8000
	b.irq = true
	b.m[0xFDFF] = 0x34
	b.m[0xFE00] = 0x12
	n := c.Step(&b)
	if n != 19 || c.PC != 0x1234 || c.MEMPTR != 0x1234 {
		t.Fatalf("IM 2 PC=%04X MEMPTR=%04X t=%d", c.PC, c.MEMPTR, n)
	}
	if c.IFF1 || c.IFF2 {
		t.Fatalf("IM 2 IFF not cleared")
	}
	if b.m[0x7FFF] != 0x01 || b.m[0x7FFE] != 0x00 || c.SP != 0x7FFE {
		t.Fatalf("IM 2 stack SP=%04X %02X %02X", c.SP, b.m[0x7FFE], b.m[0x7FFF])
	}
}

func TestEIDelaysInterrupt(t *testing.T) {
	var b ramBus
	var c CPU
	c.Reset()
	b.irq = true
	b.m[0], b.m[1], b.m[2] = 0xFB, 0x00, 0x00 // EI; NOP; NOP
	c.Step(&b)
	if c.PC != 1 || !c.IFF1 {
		t.Fatalf("after EI PC=%04X IFF %v", c.PC, c.IFF1)
	}
	c.Step(&b)
	if c.PC != 2 {
		t.Fatalf("NOP skipped, PC=%04X", c.PC)
	}
	c.Step(&b)
	if c.PC != 0x38 {
		t.Fatalf("IRQ PC=%04X", c.PC)
	}
}

func TestHalt(t *testing.T) {
	var b ramBus
	var c CPU
	c.Reset()
	b.m[0] = 0x76
	c.Step(&b)
	if !c.Halted || c.PC != 1 {
		t.Fatalf("halted %v PC %04X", c.Halted, c.PC)
	}
	r := c.R
	t0 := b.t
	c.Step(&b)
	if c.PC != 1 || b.t != t0+4 || c.R == r {
		t.Fatalf("halt nop PC=%04X dt=%d R %02X→%02X", c.PC, b.t-t0, r, c.R)
	}
	c.IFF1 = true
	b.irq = true
	c.Step(&b)
	if c.Halted || c.PC != 0x38 {
		t.Fatalf("wake PC=%04X halted %v", c.PC, c.Halted)
	}
}

func TestContendedRead(t *testing.T) {
	contend := func(addr uint16, _ int) int {
		if addr >= 0x4000 && addr <= 0x7FFF {
			return 6
		}
		return 0
	}
	var slow ramBus
	slow.t = 14335
	slow.contendMem = contend
	slow.m[0], slow.m[1], slow.m[2] = 0x3A, 0x00, 0x40
	var c CPU
	c.Reset()
	dtSlow := c.Step(&slow)
	var fast ramBus
	fast.t = 14335
	fast.contendMem = contend
	fast.m[0], fast.m[1], fast.m[2] = 0x3A, 0x00, 0x80
	c.Reset()
	dtFast := c.Step(&fast)
	if dtFast != 13 || dtSlow <= dtFast {
		t.Fatalf("contended %d uncontended %d", dtSlow, dtFast)
	}
}

func TestLDIRAndCall(t *testing.T) {
	var b ramBus
	var c CPU
	c.Reset()
	c.SetHL(0x4000)
	c.SetDE(0x5000)
	c.SetBC(3)
	b.m[0x4000], b.m[0x4001], b.m[0x4002] = 1, 2, 3
	b.m[0], b.m[1] = 0xED, 0xB0
	for c.BC() != 0 {
		c.Step(&b)
	}
	if b.m[0x5000] != 1 || b.m[0x5001] != 2 || b.m[0x5002] != 3 {
		t.Fatalf("LDIR %02X %02X %02X", b.m[0x5000], b.m[0x5001], b.m[0x5002])
	}
	c = CPU{}
	c.Reset()
	b = ramBus{}
	// CALL 0005; NOP; HALT; LD A,0x2A; RET
	copy(b.m[:], []byte{0xCD, 0x05, 0x00, 0x00, 0x76, 0x3E, 0x2A, 0xC9})
	for i := 0; i < 6 && !c.Halted; i++ {
		c.Step(&b)
	}
	if c.A != 0x2A || !c.Halted || c.PC != 5 {
		t.Fatalf("CALL A=%02X PC=%04X halted %v", c.A, c.PC, c.Halted)
	}
}

func TestUnknownED(t *testing.T) {
	var b ramBus
	var c CPU
	c.Reset()
	c.A = 0x55
	b.m[0], b.m[1] = 0xED, 0x00
	n := c.Step(&b)
	if n != 12 || c.A != 0x55 || c.PC != 2 {
		t.Fatalf("unknown ED t=%d A=%02X PC=%04X", n, c.A, c.PC)
	}
}

func TestIOContentionPattern(t *testing.T) {
	// OUT (n),A with A=0, n=0xFE: high byte not contended, bit 0 clear → one C group.
	var calls int
	var b ramBus
	b.contendIO = func(uint16, int) int {
		calls++
		return 0
	}
	var c CPU
	c.Reset()
	c.A = 0x00
	b.m[0], b.m[1] = 0xD3, 0xFE
	c.Step(&b)
	if calls != 1 {
		t.Fatalf("ULA non-high calls %d", calls)
	}
	// A=0x40, n=0x01: high byte contended, bit 0 set → four separate C cycles.
	calls = 0
	c.Reset()
	b.t = 0
	c.A = 0x40
	b.m[0], b.m[1] = 0xD3, 0x01
	c.Step(&b)
	if calls != 4 {
		t.Fatalf("contended non-ULA calls %d", calls)
	}
}
