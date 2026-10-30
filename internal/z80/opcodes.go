package z80

const (
	noIdx = 0
	useIX = 1
	useIY = 2
)

func (c *CPU) exec(bus Bus) {
	op := c.fetch(bus)
	switch op {
	case 0xDD:
		c.execIndex(bus, useIX)
	case 0xFD:
		c.execIndex(bus, useIY)
	case 0xED:
		c.execED(bus, c.fetch(bus))
	case 0xCB:
		c.execCB(bus, c.fetch(bus), c.HL(), false)
	default:
		c.execMain(bus, op, noIdx)
	}
}

func (c *CPU) execIndex(bus Bus, which int) {
	op := c.fetch(bus)
	switch op {
	case 0xDD:
		c.execIndex(bus, useIX)
	case 0xFD:
		c.execIndex(bus, useIY)
	case 0xED:
		// A preceding DD/FD does not change an ED instruction.
		c.execED(bus, c.fetch(bus))
	case 0xCB:
		d := int8(c.readOp(bus))
		op2 := c.readOp(bus)
		c.wait(bus, 2)
		base := c.IX
		if which == useIY {
			base = c.IY
		}
		addr := uint16(int(base) + int(d))
		c.MEMPTR = addr
		c.execCB(bus, op2, addr, true)
	case 0xEB:
		// EX DE,HL ignores the index prefix apart from the 4 T already counted.
		c.exDEHL()
	default:
		c.execMain(bus, op, which)
	}
}

func (c *CPU) get8(code, mode int) byte {
	switch code {
	case 0:
		return c.B
	case 1:
		return c.C
	case 2:
		return c.D
	case 3:
		return c.E
	case 4:
		switch mode {
		case useIX:
			return byte(c.IX >> 8)
		case useIY:
			return byte(c.IY >> 8)
		default:
			return c.H
		}
	case 5:
		switch mode {
		case useIX:
			return byte(c.IX)
		case useIY:
			return byte(c.IY)
		default:
			return c.L
		}
	default:
		return c.A
	}
}

func (c *CPU) set8(code, mode int, v byte) {
	switch code {
	case 0:
		c.B = v
	case 1:
		c.C = v
	case 2:
		c.D = v
	case 3:
		c.E = v
	case 4:
		switch mode {
		case useIX:
			c.IX = uint16(v)<<8 | c.IX&0xFF
		case useIY:
			c.IY = uint16(v)<<8 | c.IY&0xFF
		default:
			c.H = v
		}
	case 5:
		switch mode {
		case useIX:
			c.IX = c.IX&0xFF00 | uint16(v)
		case useIY:
			c.IY = c.IY&0xFF00 | uint16(v)
		default:
			c.L = v
		}
	case 7:
		c.A = v
	}
}

func (c *CPU) pair(code, mode int) uint16 {
	switch code {
	case 0:
		return c.BC()
	case 1:
		return c.DE()
	case 2:
		switch mode {
		case useIX:
			return c.IX
		case useIY:
			return c.IY
		default:
			return c.HL()
		}
	default:
		return c.SP
	}
}

func (c *CPU) setPair(code, mode int, v uint16) {
	switch code {
	case 0:
		c.SetBC(v)
	case 1:
		c.SetDE(v)
	case 2:
		switch mode {
		case useIX:
			c.IX = v
		case useIY:
			c.IY = v
		default:
			c.SetHL(v)
		}
	default:
		c.SP = v
	}
}

func (c *CPU) indexAddr(bus Bus, mode int) uint16 {
	d := int8(c.readOp(bus))
	base := c.IX
	if mode == useIY {
		base = c.IY
	}
	addr := uint16(int(base) + int(d))
	c.MEMPTR = addr
	return addr
}

func (c *CPU) exDEHL() {
	d, e := c.D, c.E
	c.D, c.E = c.H, c.L
	c.H, c.L = d, e
}

func (c *CPU) execMain(bus Bus, op byte, mode int) {
	switch {
	case op == 0x00:
		return
	case op == 0x76:
		c.Halted = true
	case op == 0x08: // EX AF,AF'
		c.A, c.Ap = c.Ap, c.A
		c.F, c.Fp = c.Fp, c.F
	case op == 0xD9: // EXX
		c.B, c.Bp = c.Bp, c.B
		c.C, c.Cp = c.Cp, c.C
		c.D, c.Dp = c.Dp, c.D
		c.E, c.Ep = c.Ep, c.E
		c.H, c.Hp = c.Hp, c.H
		c.L, c.Lp = c.Lp, c.L
	case op == 0xEB:
		c.exDEHL()
	case op == 0x27:
		c.daa()
	case op == 0x2F:
		c.cpl()
	case op == 0x37:
		c.scf()
	case op == 0x3F:
		c.ccf()
	case op == 0x07:
		c.rlca()
	case op == 0x0F:
		c.rrca()
	case op == 0x17:
		c.rla()
	case op == 0x1F:
		c.rra()
	case op == 0xF3:
		c.IFF1, c.IFF2 = false, false
	case op == 0xFB:
		c.IFF1, c.IFF2 = true, true
		c.intBlock = true
	case op < 0x40 && op&0xCF == 0x01:
		// LD dd,nn
		c.setPair(int(op>>4), mode, c.readWord(bus))
	case op < 0x40 && op&0xCF == 0x03:
		c.wait(bus, 2)
		p := int(op >> 4)
		c.setPair(p, mode, c.pair(p, mode)+1)
	case op < 0x40 && op&0xCF == 0x0B:
		c.wait(bus, 2)
		p := int(op >> 4)
		c.setPair(p, mode, c.pair(p, mode)-1)
	case op < 0x40 && op&0xCF == 0x09:
		p := int(op >> 4)
		c.wait(bus, 7)
		hl := c.pair(2, mode)
		c.setPair(2, mode, c.add16(hl, c.pair(p, mode), false, false))
	case op < 0x40 && op&0xC7 == 0x04:
		reg := int((op >> 3) & 7)
		if reg == 6 {
			c.incMem(bus, mode, true)
		} else {
			c.set8(reg, mode, c.inc8(c.get8(reg, mode)))
		}
	case op < 0x40 && op&0xC7 == 0x05:
		reg := int((op >> 3) & 7)
		if reg == 6 {
			c.incMem(bus, mode, false)
		} else {
			c.set8(reg, mode, c.dec8(c.get8(reg, mode)))
		}
	case op < 0x40 && op&0xC7 == 0x06:
		reg := int((op >> 3) & 7)
		if reg == 6 {
			c.ldMemN(bus, mode)
		} else {
			c.set8(reg, mode, c.readOp(bus))
		}
	case op == 0x02 || op == 0x12:
		// LD (BC/DE),A
		addr := c.BC()
		if op == 0x12 {
			addr = c.DE()
		}
		c.writeAbs(bus, addr, c.A, 3)
		c.MEMPTR = uint16(c.A)<<8 | ((addr + 1) & 0xFF)
	case op == 0x0A || op == 0x1A:
		addr := c.BC()
		if op == 0x1A {
			addr = c.DE()
		}
		c.A = c.readAbs(bus, addr, 3)
		c.MEMPTR = addr + 1
	case op == 0x22:
		addr := c.readWord(bus)
		c.MEMPTR = addr + 1
		hl := c.pair(2, mode)
		c.writeAbs(bus, addr, byte(hl), 3)
		c.writeAbs(bus, addr+1, byte(hl>>8), 3)
	case op == 0x2A:
		addr := c.readWord(bus)
		c.MEMPTR = addr + 1
		lo := c.readAbs(bus, addr, 3)
		hi := c.readAbs(bus, addr+1, 3)
		c.setPair(2, mode, uint16(lo)|uint16(hi)<<8)
	case op == 0x32:
		addr := c.readWord(bus)
		c.writeAbs(bus, addr, c.A, 3)
		c.MEMPTR = uint16(c.A)<<8 | ((addr + 1) & 0xFF)
	case op == 0x3A:
		addr := c.readWord(bus)
		c.A = c.readAbs(bus, addr, 3)
		c.MEMPTR = addr + 1
	case op == 0x10:
		c.djnz(bus)
	case op == 0x18:
		c.jr(bus, true)
	case op == 0x20:
		c.jr(bus, c.F&FlagZ == 0)
	case op == 0x28:
		c.jr(bus, c.F&FlagZ != 0)
	case op == 0x30:
		c.jr(bus, c.F&FlagC == 0)
	case op == 0x38:
		c.jr(bus, c.F&FlagC != 0)
	case op >= 0x40 && op < 0x80:
		c.ldRR(bus, op, mode)
	case op >= 0x80 && op < 0xC0:
		c.aluR(bus, op, mode)
	default:
		c.execCtrl(bus, op, mode)
	}
}

func (c *CPU) readWord(bus Bus) uint16 {
	lo := c.readOp(bus)
	hi := c.readOp(bus)
	return uint16(lo) | uint16(hi)<<8
}

func (c *CPU) cpl() {
	c.A = ^c.A
	f := c.F & (FlagS | FlagZ | FlagP | FlagC)
	f |= FlagH | FlagN
	f |= c.A & (FlagX | FlagY)
	c.F = f
}

func (c *CPU) scf() {
	f := c.F & (FlagS | FlagZ | FlagP)
	f |= FlagC
	f |= c.A & (FlagX | FlagY)
	c.F = f
}

func (c *CPU) ccf() {
	old := c.F & FlagC
	f := c.F & (FlagS | FlagZ | FlagP)
	if old == 0 {
		f |= FlagC
	}
	if old != 0 {
		f |= FlagH
	}
	f |= c.A & (FlagX | FlagY)
	c.F = f
}

func (c *CPU) rlca() {
	cy := c.A >> 7
	c.A = c.A<<1 | cy
	f := c.F & (FlagS | FlagZ | FlagP)
	if cy != 0 {
		f |= FlagC
	}
	f |= c.A & (FlagX | FlagY)
	c.F = f
}

func (c *CPU) rrca() {
	cy := c.A & 1
	c.A = c.A>>1 | cy<<7
	f := c.F & (FlagS | FlagZ | FlagP)
	if cy != 0 {
		f |= FlagC
	}
	f |= c.A & (FlagX | FlagY)
	c.F = f
}

func (c *CPU) rla() {
	cy := c.A >> 7
	c.A = c.A<<1 | c.F&FlagC
	f := c.F & (FlagS | FlagZ | FlagP)
	if cy != 0 {
		f |= FlagC
	}
	f |= c.A & (FlagX | FlagY)
	c.F = f
}

func (c *CPU) rra() {
	cy := c.A & 1
	c.A = c.A>>1 | (c.F&FlagC)<<7
	f := c.F & (FlagS | FlagZ | FlagP)
	if cy != 0 {
		f |= FlagC
	}
	f |= c.A & (FlagX | FlagY)
	c.F = f
}

func (c *CPU) incMem(bus Bus, mode int, inc bool) {
	addr := c.HL()
	if mode != noIdx {
		addr = c.indexAddr(bus, mode)
		c.wait(bus, 5)
	}
	v := c.readAbs(bus, addr, 3)
	c.wait(bus, 1)
	if inc {
		v = c.inc8(v)
	} else {
		v = c.dec8(v)
	}
	c.writeAbs(bus, addr, v, 3)
}

func (c *CPU) ldMemN(bus Bus, mode int) {
	addr := c.HL()
	extra := 0
	if mode != noIdx {
		addr = c.indexAddr(bus, mode)
		extra = 2
	}
	n := c.readOp(bus)
	c.wait(bus, extra)
	c.writeAbs(bus, addr, n, 3)
}

func (c *CPU) djnz(bus Bus) {
	c.B--
	rel := int8(c.readOp(bus))
	c.wait(bus, 1)
	if c.B != 0 {
		c.wait(bus, 5)
		c.PC = uint16(int32(c.PC) + int32(rel))
		c.MEMPTR = c.PC
	}
}

func (c *CPU) jr(bus Bus, take bool) {
	rel := int8(c.readOp(bus))
	if take {
		c.wait(bus, 5)
		c.PC = uint16(int32(c.PC) + int32(rel))
		c.MEMPTR = c.PC
	}
}

func (c *CPU) ldRR(bus Bus, op byte, mode int) {
	dst := int((op >> 3) & 7)
	src := int(op & 7)
	if mode != noIdx && (dst == 6 || src == 6) {
		// (IX/IY+d) replaces (HL). The other operand stays the real
		// register, including H and L, and the index register is unchanged.
		addr := c.indexAddr(bus, mode)
		c.wait(bus, 5)
		if src == 6 {
			c.set8(dst, noIdx, c.readAbs(bus, addr, 3))
		} else {
			c.writeAbs(bus, addr, c.get8(src, noIdx), 3)
		}
		return
	}
	if src == 6 {
		c.set8(dst, noIdx, c.readAbs(bus, c.HL(), 3))
		return
	}
	if dst == 6 {
		c.writeAbs(bus, c.HL(), c.get8(src, noIdx), 3)
		return
	}
	c.set8(dst, mode, c.get8(src, mode))
}

func (c *CPU) aluR(bus Bus, op byte, mode int) {
	kind := (op >> 3) & 7
	src := int(op & 7)
	var val byte
	if src == 6 {
		addr := c.HL()
		if mode != noIdx {
			addr = c.indexAddr(bus, mode)
			c.wait(bus, 5)
		}
		val = c.readAbs(bus, addr, 3)
	} else {
		val = c.get8(src, mode)
	}
	c.alu8(kind, val)
}

func (c *CPU) cond(cc byte) bool {
	switch cc {
	case 0:
		return c.F&FlagZ == 0
	case 1:
		return c.F&FlagZ != 0
	case 2:
		return c.F&FlagC == 0
	case 3:
		return c.F&FlagC != 0
	case 4:
		return c.F&FlagP == 0
	case 5:
		return c.F&FlagP != 0
	case 6:
		return c.F&FlagS == 0
	default:
		return c.F&FlagS != 0
	}
}

func (c *CPU) execCtrl(bus Bus, op byte, mode int) {
	switch op {
	case 0xC0, 0xC8, 0xD0, 0xD8, 0xE0, 0xE8, 0xF0, 0xF8:
		c.wait(bus, 1)
		if c.cond((op >> 3) & 7) {
			c.PC = c.pop(bus)
			c.MEMPTR = c.PC
		}
	case 0xC9:
		c.PC = c.pop(bus)
		c.MEMPTR = c.PC
	case 0xC1, 0xD1, 0xE1, 0xF1:
		v := c.pop(bus)
		switch op {
		case 0xC1:
			c.SetBC(v)
		case 0xD1:
			c.SetDE(v)
		case 0xE1:
			c.setPair(2, mode, v)
		default:
			c.SetAF(v)
		}
	case 0xC5, 0xD5, 0xE5, 0xF5:
		var v uint16
		switch op {
		case 0xC5:
			v = c.BC()
		case 0xD5:
			v = c.DE()
		case 0xE5:
			v = c.pair(2, mode)
		default:
			v = c.AF()
		}
		c.push(bus, v)
	case 0xC2, 0xCA, 0xD2, 0xDA, 0xE2, 0xEA, 0xF2, 0xFA, 0xC3:
		addr := c.readWord(bus)
		c.MEMPTR = addr
		if op == 0xC3 || c.cond((op>>3)&7) {
			c.PC = addr
		}
	case 0xC4, 0xCC, 0xD4, 0xDC, 0xE4, 0xEC, 0xF4, 0xFC, 0xCD:
		addr := c.readWord(bus)
		c.MEMPTR = addr
		if op == 0xCD || c.cond((op>>3)&7) {
			c.push(bus, c.PC)
			c.PC = addr
		}
	case 0xC6, 0xCE, 0xD6, 0xDE, 0xE6, 0xEE, 0xF6, 0xFE:
		n := c.readOp(bus)
		kind := (op >> 3) & 7
		c.alu8(kind, n)
	case 0xC7, 0xCF, 0xD7, 0xDF, 0xE7, 0xEF, 0xF7, 0xFF:
		addr := uint16(op & 0x38)
		c.push(bus, c.PC)
		c.PC = addr
		c.MEMPTR = addr
	case 0xE3: // EX (SP),HL/IX
		lo := c.readAbs(bus, c.SP, 3)
		hi := c.readAbs(bus, c.SP+1, 3)
		c.MEMPTR = c.SP + 1
		c.wait(bus, 1)
		hl := c.pair(2, mode)
		c.writeAbs(bus, c.SP+1, byte(hl>>8), 3)
		c.writeAbs(bus, c.SP, byte(hl), 3)
		c.wait(bus, 2)
		c.setPair(2, mode, uint16(lo)|uint16(hi)<<8)
	case 0xE9: // JP (HL/IX)
		c.PC = c.pair(2, mode)
	case 0xF9: // LD SP,HL/IX
		c.wait(bus, 2)
		c.SP = c.pair(2, mode)
	case 0xD3:
		n := c.readOp(bus)
		port := uint16(c.A)<<8 | uint16(n)
		c.MEMPTR = uint16(c.A)<<8 | uint16(n+1)
		c.ioCycles(bus, port, true, c.A)
	case 0xDB:
		n := c.readOp(bus)
		port := uint16(c.A)<<8 | uint16(n)
		c.MEMPTR = port + 1
		c.A = c.ioCycles(bus, port, false, 0)
	default:
		// Every main-set opcode is defined. This is a prefix handled elsewhere.
	}
}

func (c *CPU) execCB(bus Bus, op byte, addr uint16, indexed bool) {
	reg := int(op & 7)
	mem := indexed || reg == 6
	var val, xy byte
	if mem {
		if indexed {
			xy = byte(c.MEMPTR >> 8)
		} else {
			xy = byte(addr >> 8)
		}
		val = c.readAbs(bus, addr, 3)
		c.wait(bus, 1)
	} else {
		val = c.get8(reg, noIdx)
		xy = val
	}
	group := op >> 6
	if group == 1 {
		c.bit((op>>3)&7, val, xy)
		return
	}
	if group == 0 {
		val, c.F = c.rotMem((op>>3)&7, val)
	} else if group == 2 {
		val &^= 1 << ((op >> 3) & 7)
	} else {
		val |= 1 << ((op >> 3) & 7)
	}
	if mem {
		c.writeAbs(bus, addr, val, 3)
		if indexed && reg != 6 {
			c.set8(reg, noIdx, val)
		}
		return
	}
	c.set8(reg, noIdx, val)
}

func (c *CPU) execED(bus Bus, op byte) {
	switch op {
	case 0x47: // LD I,A
		c.wait(bus, 1)
		c.I = c.A
	case 0x4F: // LD R,A
		c.wait(bus, 1)
		c.R = c.A
	case 0x57: // LD A,I
		c.wait(bus, 1)
		c.A = c.I
		c.ldAIR(c.A)
	case 0x5F: // LD A,R
		c.wait(bus, 1)
		c.A = c.R
		c.ldAIR(c.A)
	case 0x67:
		c.rld(bus, false)
	case 0x6F:
		c.rld(bus, true)
	default:
		if op&0xC7 == 0x44 {
			// NEG and its undocumented aliases: A = 0-A.
			c.neg()
			return
		}
		switch {
		case op == 0x4D:
			c.retn(bus, false)
		case op&0xC7 == 0x45:
			c.retn(bus, true)
		case c.edIM(op):
			return
		case op&0xC7 == 0x40 && op != 0x70:
			c.inC(bus, op)
		case op == 0x70:
			c.inC(bus, op)
		case op&0xC7 == 0x41:
			c.outC(bus, op)
		case c.edBlock(bus, op):
			return
		case c.ed16(bus, op):
			return
		default:
			// Undocumented ED: 8 T body + the 4 T prefix already fetched = 12.
			c.wait(bus, 4)
		}
	}
}

func (c *CPU) neg() {
	a := c.A
	// SUB from 0. alu8 above was a mistaken call; do it cleanly.
	c.A = 0
	c.alu8(2, a)
}

func (c *CPU) ldAIR(v byte) {
	f := flagsSZP(v) &^ FlagP
	f |= c.F & FlagC
	if c.IFF2 {
		f |= FlagP
	}
	c.F = f
}

func (c *CPU) rld(bus Bus, left bool) {
	hl := c.HL()
	mem := c.readAbs(bus, hl, 3)
	c.wait(bus, 4)
	a := c.A
	if left {
		c.A = (a & 0xF0) | (mem >> 4)
		mem = mem<<4 | a&0x0F
	} else {
		c.A = (a & 0xF0) | (mem & 0x0F)
		mem = mem>>4 | (a&0x0F)<<4
	}
	c.writeAbs(bus, hl, mem, 3)
	f := flagsSZP(c.A)
	f |= c.F & FlagC
	c.F = f
	c.MEMPTR = hl + 1
}

func (c *CPU) retn(bus Bus, copyIFF bool) {
	c.PC = c.pop(bus)
	c.MEMPTR = c.PC
	if copyIFF {
		c.IFF1 = c.IFF2
	}
}

func (c *CPU) edIM(op byte) bool {
	switch op {
	case 0x46, 0x4E, 0x66, 0x6E:
		c.IM = 0
	case 0x56, 0x76:
		c.IM = 1
	case 0x5E, 0x7E:
		c.IM = 2
	default:
		return false
	}
	return true
}

func (c *CPU) inC(bus Bus, op byte) {
	reg := int((op >> 3) & 7)
	c.MEMPTR = c.BC() + 1
	v := c.ioCycles(bus, c.BC(), false, 0)
	f := flagsSZP(v)
	f |= c.F & FlagC
	c.F = f
	if reg != 6 {
		c.set8(reg, noIdx, v)
	}
}

func (c *CPU) outC(bus Bus, op byte) {
	reg := int((op >> 3) & 7)
	val := byte(0)
	if reg != 6 {
		val = c.get8(reg, noIdx)
	}
	c.MEMPTR = c.BC() + 1
	c.ioCycles(bus, c.BC(), true, val)
}

func (c *CPU) ed16(bus Bus, op byte) bool {
	reg := int((op >> 4) & 3)
	switch op & 0xCF {
	case 0x42: // SBC HL,ss
		c.wait(bus, 7)
		c.SetHL(c.add16(c.HL(), c.pair(reg, noIdx), false, true))
	case 0x4A: // ADC HL,ss
		c.wait(bus, 7)
		c.SetHL(c.add16(c.HL(), c.pair(reg, noIdx), true, false))
	case 0x43: // LD (nn),dd
		addr := c.readWord(bus)
		c.MEMPTR = addr + 1
		v := c.pair(reg, noIdx)
		c.writeAbs(bus, addr, byte(v), 3)
		c.writeAbs(bus, addr+1, byte(v>>8), 3)
	case 0x4B: // LD dd,(nn)
		addr := c.readWord(bus)
		c.MEMPTR = addr + 1
		lo := c.readAbs(bus, addr, 3)
		hi := c.readAbs(bus, addr+1, 3)
		c.setPair(reg, noIdx, uint16(lo)|uint16(hi)<<8)
	default:
		return false
	}
	return true
}

func (c *CPU) edBlock(bus Bus, op byte) bool {
	switch op {
	case 0xA0, 0xB0:
		c.ldBlock(bus, false, op == 0xB0)
	case 0xA8, 0xB8:
		c.ldBlock(bus, true, op == 0xB8)
	case 0xA1, 0xB1:
		c.cpBlock(bus, false, op == 0xB1)
	case 0xA9, 0xB9:
		c.cpBlock(bus, true, op == 0xB9)
	case 0xA2, 0xB2:
		c.inBlock(bus, false, op == 0xB2)
	case 0xAA, 0xBA:
		c.inBlock(bus, true, op == 0xBA)
	case 0xA3, 0xB3:
		c.outBlock(bus, false, op == 0xB3)
	case 0xAB, 0xBB:
		c.outBlock(bus, true, op == 0xBB)
	default:
		return false
	}
	return true
}

func (c *CPU) ldBlock(bus Bus, dec, rep bool) {
	val := c.readAbs(bus, c.HL(), 3)
	c.writeAbs(bus, c.DE(), val, 3)
	c.wait(bus, 2)
	if dec {
		c.SetHL(c.HL() - 1)
		c.SetDE(c.DE() - 1)
	} else {
		c.SetHL(c.HL() + 1)
		c.SetDE(c.DE() + 1)
	}
	c.SetBC(c.BC() - 1)
	sum := c.A + val
	f := c.F & (FlagS | FlagZ | FlagC)
	if c.BC() != 0 {
		f |= FlagP
	}
	if sum&0x08 != 0 {
		f |= FlagX
	}
	if sum&0x02 != 0 {
		f |= FlagY
	}
	c.F = f
	if rep && c.BC() != 0 {
		c.wait(bus, 5)
		c.PC -= 2
		c.MEMPTR = c.PC + 1
	}
}

func (c *CPU) cpBlock(bus Bus, dec, rep bool) {
	val := c.readAbs(bus, c.HL(), 3)
	c.wait(bus, 5)
	if dec {
		c.SetHL(c.HL() - 1)
	} else {
		c.SetHL(c.HL() + 1)
	}
	c.SetBC(c.BC() - 1)
	diff := int(c.A) - int(val)
	r := byte(diff)
	f := c.F & FlagC
	f |= FlagN
	if r == 0 {
		f |= FlagZ
	}
	if r&0x80 != 0 {
		f |= FlagS
	}
	if int(c.A&0x0F)-int(val&0x0F) < 0 {
		f |= FlagH
	}
	if c.BC() != 0 {
		f |= FlagP
	}
	n := r
	if f&FlagH != 0 {
		n--
	}
	if n&0x08 != 0 {
		f |= FlagX
	}
	if n&0x02 != 0 {
		f |= FlagY
	}
	c.F = f
	c.MEMPTR++
	if rep && c.BC() != 0 && r != 0 {
		c.wait(bus, 5)
		c.PC -= 2
		c.MEMPTR = c.PC + 1
	}
}

func (c *CPU) inBlock(bus Bus, dec, rep bool) {
	c.wait(bus, 1)
	v := c.ioCycles(bus, c.BC(), false, 0)
	c.writeAbs(bus, c.HL(), v, 3)
	c.B--
	if dec {
		c.SetHL(c.HL() - 1)
	} else {
		c.SetHL(c.HL() + 1)
	}
	c.MEMPTR = c.BC() + 1
	f := c.F & FlagC
	f |= FlagN
	if c.B == 0 {
		f |= FlagZ
	}
	f |= c.B & (FlagX | FlagY)
	if c.B&0x80 != 0 {
		f |= FlagS
	}
	c.F = f
	if rep && c.B != 0 {
		c.wait(bus, 5)
		c.PC -= 2
	}
}

func (c *CPU) outBlock(bus Bus, dec, rep bool) {
	c.wait(bus, 1)
	v := c.readAbs(bus, c.HL(), 3)
	c.B--
	c.ioCycles(bus, c.BC(), true, v)
	if dec {
		c.SetHL(c.HL() - 1)
	} else {
		c.SetHL(c.HL() + 1)
	}
	c.MEMPTR = c.BC() + 1
	f := c.F & FlagC
	f |= FlagN
	if c.B == 0 {
		f |= FlagZ
	}
	f |= c.B & (FlagX | FlagY)
	if c.B&0x80 != 0 {
		f |= FlagS
	}
	c.F = f
	if rep && c.B != 0 {
		c.wait(bus, 5)
		c.PC -= 2
	}
}
