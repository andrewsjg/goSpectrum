package machine

// ayChip is the AY-3-8912 in the 128K. The chip clock is half the CPU
// clock, and the tone and noise counters step every 16 of those clocks,
// which is one step every 32 CPU T-states. The envelope steps 16 times
// slower than that.
type ayChip struct {
	reg    [16]byte
	sel    int
	div    int
	envdiv int
	cnt    [3]int
	bit    [3]bool
	ncnt   int
	nbit   bool
	lfsr   int
	epos   int
	ehold  bool
	ealt   bool
	held   int
	level  int16
}

// ayLog is the AY amplitude curve, scaled so three voices fit beside the beeper.
var ayLog = [16]int16{
	0, 16, 23, 33, 46, 65, 92, 130,
	184, 260, 368, 520, 735, 1039, 1468, 2075,
}

func (a *ayChip) reset() {
	*a = ayChip{lfsr: 0x1FFFF}
	a.reg[7] = 0xFF
}

func (a *ayChip) selectReg(v byte) { a.sel = int(v & 0x0F) }

func (a *ayChip) write(v byte) {
	if a.sel > 13 {
		return
	}
	a.reg[a.sel] = v
	if a.sel == 13 {
		a.epos = 0
		a.ehold = false
		a.ealt = false
		a.envdiv = 0
	}
}

func (a *ayChip) read() byte {
	if a.sel > 13 {
		return 0xFF
	}
	return a.reg[a.sel]
}

func (a *ayChip) load(sel byte, reg [16]byte) {
	a.reset()
	a.sel = int(sel & 0x0F)
	copy(a.reg[:14], reg[:14])
}

func (a *ayChip) tick() {
	a.div++
	if a.div >= 32 {
		a.div = 0
		a.stepTone()
		a.envdiv++
		if a.envdiv >= 16 {
			a.envdiv = 0
			a.stepEnv()
		}
	}
	a.mix()
}

func (a *ayChip) period(ch int) int {
	p := int(a.reg[ch*2]) | int(a.reg[ch*2+1]&0x0F)<<8
	if p < 1 {
		return 1
	}
	return p
}

func (a *ayChip) stepTone() {
	for ch := 0; ch < 3; ch++ {
		a.cnt[ch]--
		if a.cnt[ch] <= 0 {
			a.cnt[ch] = a.period(ch)
			a.bit[ch] = !a.bit[ch]
		}
	}
	np := int(a.reg[6] & 0x1F)
	if np < 1 {
		np = 1
	}
	a.ncnt--
	if a.ncnt <= 0 {
		a.ncnt = np
		bit := (a.lfsr ^ (a.lfsr >> 3)) & 1
		a.lfsr = (a.lfsr >> 1) | (bit << 16)
		if a.lfsr == 0 {
			a.lfsr = 0x1FFFF
		}
		a.nbit = a.lfsr&1 != 0
	}
}

func (a *ayChip) stepEnv() {
	if a.ehold {
		return
	}
	a.epos++
	if a.epos <= 15 {
		return
	}
	shape := a.reg[13] & 0x0F
	if shape&0x08 == 0 {
		a.ehold = true
		a.held = 0
		return
	}
	if shape&0x01 != 0 {
		a.ehold = true
		attack := shape&0x04 != 0
		alt := shape&0x02 != 0
		a.held = 0
		if attack != alt {
			a.held = 15
		}
		return
	}
	a.epos = 0
	if shape&0x02 != 0 {
		a.ealt = !a.ealt
	}
}

func (a *ayChip) envAmp() int {
	if a.ehold {
		return a.held
	}
	pos := a.epos
	if pos > 15 {
		pos = 15
	}
	attack := a.reg[13]&0x04 != 0
	if attack != a.ealt {
		return pos
	}
	return 15 - pos
}

func (a *ayChip) mix() {
	mix := a.reg[7]
	sum := 0
	for ch := 0; ch < 3; ch++ {
		on := true
		if mix&(1<<uint(ch)) == 0 {
			on = a.bit[ch]
		}
		if mix&(8<<uint(ch)) == 0 {
			on = on && a.nbit
		}
		if !on {
			continue
		}
		amp := int(a.reg[8+ch])
		vol := amp & 15
		if amp&16 != 0 {
			vol = a.envAmp()
		}
		sum += int(ayLog[vol])
	}
	if sum > 32767 {
		sum = 32767
	}
	a.level = int16(sum)
}
