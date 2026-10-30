package z80

func evenParity(v byte) bool {
	v ^= v >> 4
	v ^= v >> 2
	v ^= v >> 1
	return v&1 == 0
}

func flagsSZP(v byte) byte {
	var f byte
	if v == 0 {
		f |= FlagZ
	}
	if v&0x80 != 0 {
		f |= FlagS
	}
	if evenParity(v) {
		f |= FlagP
	}
	f |= v & (FlagX | FlagY)
	return f
}

func (c *CPU) setSZP(v byte, keepC, n, h bool) {
	f := flagsSZP(v)
	if keepC && c.F&FlagC != 0 {
		f |= FlagC
	}
	if n {
		f |= FlagN
	}
	if h {
		f |= FlagH
	}
	c.F = f
}

func (c *CPU) inc8(v byte) byte {
	r := v + 1
	f := c.F & FlagC
	if r == 0 {
		f |= FlagZ
	}
	if r&0x80 != 0 {
		f |= FlagS
	}
	if v == 0x7F {
		f |= FlagP
	}
	if v&0x0F == 0x0F {
		f |= FlagH
	}
	f |= r & (FlagX | FlagY)
	c.F = f
	return r
}

func (c *CPU) dec8(v byte) byte {
	r := v - 1
	f := c.F & FlagC
	f |= FlagN
	if r == 0 {
		f |= FlagZ
	}
	if r&0x80 != 0 {
		f |= FlagS
	}
	if v == 0x80 {
		f |= FlagP
	}
	if v&0x0F == 0 {
		f |= FlagH
	}
	f |= r & (FlagX | FlagY)
	c.F = f
	return r
}

// alu8 applies an 8-bit ALU op. kind: 0 ADD, 1 ADC, 2 SUB, 3 SBC, 4 AND, 5 XOR, 6 OR, 7 CP.
func (c *CPU) alu8(kind, b byte) {
	switch kind {
	case 4:
		c.A &= b
		c.F = flagsSZP(c.A) | FlagH
		return
	case 5:
		c.A ^= b
		c.F = flagsSZP(c.A)
		return
	case 6:
		c.A |= b
		c.F = flagsSZP(c.A)
		return
	}
	sub := kind == 2 || kind == 3 || kind == 7
	carryIn := byte(0)
	if (kind == 1 || kind == 3) && c.F&FlagC != 0 {
		carryIn = 1
	}
	a := c.A
	var res int
	if sub {
		res = int(a) - int(b) - int(carryIn)
	} else {
		res = int(a) + int(b) + int(carryIn)
	}
	r := byte(res)
	var f byte
	if r == 0 {
		f |= FlagZ
	}
	if r&0x80 != 0 {
		f |= FlagS
	}
	if sub {
		f |= FlagN
		if res < 0 {
			f |= FlagC
		}
		if int(a&0x0F)-int(b&0x0F)-int(carryIn) < 0 {
			f |= FlagH
		}
		if (a^b)&(a^r)&0x80 != 0 {
			f |= FlagP
		}
	} else {
		if res > 0xFF {
			f |= FlagC
		}
		if int(a&0x0F)+int(b&0x0F)+int(carryIn) > 0x0F {
			f |= FlagH
		}
		if (a^b^0x80)&(a^r)&0x80 != 0 {
			f |= FlagP
		}
	}
	if kind == 7 {
		// CP keeps A and takes X/Y from the operand, not the result.
		f = (f &^ (FlagX | FlagY)) | (b & (FlagX | FlagY))
	} else {
		f |= r & (FlagX | FlagY)
		c.A = r
	}
	c.F = f
}

func (c *CPU) daa() {
	a := c.A
	f := c.F
	var adj byte
	cy := f&FlagC != 0
	hc := false
	if f&FlagN == 0 {
		if f&FlagH != 0 || a&0x0F > 9 {
			adj = 0x06
		}
		if f&FlagC != 0 || a > 0x99 {
			adj += 0x60
			cy = true
		}
		hc = a&0x0F > 9
		a += adj
	} else {
		if f&FlagH != 0 {
			adj = 0x06
		}
		if f&FlagC != 0 {
			adj += 0x60
		}
		hc = f&FlagH != 0 && a&0x0F < 6
		a -= adj
	}
	c.A = a
	nf := flagsSZP(a) | (f & FlagN)
	if cy {
		nf |= FlagC
	}
	if hc {
		nf |= FlagH
	}
	c.F = nf
}

func (c *CPU) add16(a, b uint16, adc, sbc bool) uint16 {
	c.MEMPTR = a + 1
	cin := 0
	if (adc || sbc) && c.F&FlagC != 0 {
		cin = 1
	}
	var res int
	if sbc {
		res = int(a) - int(b) - cin
	} else {
		res = int(a) + int(b) + cin
	}
	r := uint16(res)
	if !adc && !sbc {
		f := c.F & (FlagS | FlagZ | FlagP)
		if res > 0xFFFF {
			f |= FlagC
		}
		if int(a&0x0FFF)+int(b&0x0FFF) > 0x0FFF {
			f |= FlagH
		}
		f |= byte(r>>8) & (FlagX | FlagY)
		c.F = f
		return r
	}
	var f byte
	if r&0x8000 != 0 {
		f |= FlagS
	}
	if r == 0 {
		f |= FlagZ
	}
	f |= byte(r>>8) & (FlagX | FlagY)
	if sbc {
		f |= FlagN
		if res < 0 {
			f |= FlagC
		}
		if int(a&0x0FFF)-int(b&0x0FFF)-cin < 0 {
			f |= FlagH
		}
		if (a^b)&(a^r)&0x8000 != 0 {
			f |= FlagP
		}
	} else {
		if res > 0xFFFF {
			f |= FlagC
		}
		if int(a&0x0FFF)+int(b&0x0FFF)+cin > 0x0FFF {
			f |= FlagH
		}
		if (^(a ^ b))&(a^r)&0x8000 != 0 {
			f |= FlagP
		}
	}
	c.F = f
	return r
}

func (c *CPU) bit(n, v, xy byte) {
	f := c.F & FlagC
	f |= FlagH
	if v&(1<<n) == 0 {
		f |= FlagZ | FlagP
	} else if n == 7 {
		f |= FlagS
	}
	f |= xy & (FlagX | FlagY)
	c.F = f
}

func (c *CPU) rotMem(kind, v byte) (byte, byte) {
	oldC := c.F & FlagC
	var cy byte
	switch kind {
	case 0: // RLC
		cy = v >> 7
		v = v<<1 | cy
	case 1: // RRC
		cy = v & 1
		v = v>>1 | cy<<7
	case 2: // RL
		cy = v >> 7
		v = v<<1 | oldC
	case 3: // RR
		cy = v & 1
		v = v>>1 | oldC<<7
	case 4: // SLA
		cy = v >> 7
		v <<= 1
	case 5: // SRA
		cy = v & 1
		v = v>>1 | v&0x80
	case 6: // SLL, undocumented: SLA with bit 0 set
		cy = v >> 7
		v = v<<1 | 1
	default: // SRL
		cy = v & 1
		v >>= 1
	}
	f := flagsSZP(v)
	if cy != 0 {
		f |= FlagC
	}
	return v, f
}
