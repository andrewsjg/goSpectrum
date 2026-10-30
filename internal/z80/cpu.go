// Package z80 is a cycle-stepped Z80. Memory and I/O go through Bus so the
// machine can insert contention; those waits count as T-states.
package z80

// Flag bits. X and Y are the undocumented copies of result bits 3 and 5.
const (
	FlagC byte = 1 << 0
	FlagN byte = 1 << 1
	FlagP byte = 1 << 2
	FlagX byte = 1 << 3
	FlagH byte = 1 << 4
	FlagY byte = 1 << 5
	FlagZ byte = 1 << 6
	FlagS byte = 1 << 7
)

// Bus is the memory, I/O, and clock the CPU steps against.
// Contend* is called with the T-state at which the access begins and returns
// extra wait states. Advance moves the clock, including those waits.
type Bus interface {
	T() int
	Advance(n int)
	MemRead(addr uint16) byte
	MemWrite(addr uint16, v byte)
	IORead(port uint16) byte
	IOWrite(port uint16, v byte)
	ContendMem(addr uint16, t int) int
	ContendIO(port uint16, t int) int
	InterruptAsserted() bool
	InterruptAck() byte
}

// CPU is one Z80. Register fields are exported for tests and snapshot restore.
type CPU struct {
	A, F   byte
	B, C   byte
	D, E   byte
	H, L   byte
	Ap, Fp byte
	Bp, Cp byte
	Dp, Ep byte
	Hp, Lp byte
	IX, IY uint16
	SP, PC uint16
	I, R   byte
	IFF1   bool
	IFF2   bool
	IM     byte
	MEMPTR uint16
	Halted bool

	intBlock bool // EI suppresses the interrupt for one instruction
}

// Reset is the 48K power-on state: PC, I, R, IFF, and IM cleared, SP at 0xFFFF.
func (c *CPU) Reset() {
	*c = CPU{SP: 0xFFFF}
}

// Step runs one instruction, or a halted NOP, or an accepted interrupt.
// INT is sampled only on this boundary. The returned count includes waits.
func (c *CPU) Step(bus Bus) int {
	t0 := bus.T()
	blocked := c.intBlock
	c.intBlock = false
	irq := !blocked && c.IFF1 && bus.InterruptAsserted()
	if c.Halted {
		if irq {
			c.Halted = false
			c.acceptIRQ(bus)
		} else {
			c.incR()
			bus.Advance(4)
		}
		return bus.T() - t0
	}
	if irq {
		c.acceptIRQ(bus)
		return bus.T() - t0
	}
	c.exec(bus)
	return bus.T() - t0
}

func (c *CPU) acceptIRQ(bus Bus) {
	c.IFF1, c.IFF2 = false, false
	c.Halted = false
	c.incR()
	if c.IM == 2 {
		// 6 + two vector reads + push = 19 T. RET in the handler needs the old PC.
		bus.Advance(6)
		vec := bus.InterruptAck()
		addr := uint16(c.I)<<8 | uint16(vec)
		lo := c.readAbs(bus, addr, 3)
		hi := c.readAbs(bus, addr+1, 3)
		ret := c.PC
		c.push(bus, ret)
		c.PC = uint16(lo) | uint16(hi)<<8
		c.MEMPTR = c.PC
		return
	}
	// IM 0 and IM 1. Interrupt acknowledge reads 0xFF on an unexpanded 48K,
	// which is RST 0x38. push contributes 7 T; 6 + 7 = 13.
	bus.Advance(6)
	c.push(bus, c.PC)
	c.PC = 0x0038
	c.MEMPTR = c.PC
}

func (c *CPU) incR() {
	c.R = (c.R & 0x80) | ((c.R + 1) & 0x7F)
}

func (c *CPU) fetch(bus Bus) byte {
	c.contend(bus, c.PC, true)
	b := bus.MemRead(c.PC)
	c.PC++
	c.incR()
	bus.Advance(4)
	return b
}

func (c *CPU) readOp(bus Bus) byte {
	c.contend(bus, c.PC, true)
	b := bus.MemRead(c.PC)
	c.PC++
	bus.Advance(3)
	return b
}

func (c *CPU) readAbs(bus Bus, addr uint16, n int) byte {
	c.contend(bus, addr, true)
	b := bus.MemRead(addr)
	bus.Advance(n)
	return b
}

func (c *CPU) writeAbs(bus Bus, addr uint16, v byte, n int) {
	c.contend(bus, addr, true)
	bus.Advance(n)
	bus.MemWrite(addr, v)
}

func (c *CPU) contend(bus Bus, addr uint16, mem bool) {
	d := 0
	if mem {
		d = bus.ContendMem(addr, bus.T())
	}
	if d != 0 {
		bus.Advance(d)
	}
}

func (c *CPU) wait(bus Bus, n int) {
	if n > 0 {
		bus.Advance(n)
	}
}

func (c *CPU) HL() uint16 { return uint16(c.H)<<8 | uint16(c.L) }
func (c *CPU) BC() uint16 { return uint16(c.B)<<8 | uint16(c.C) }
func (c *CPU) DE() uint16 { return uint16(c.D)<<8 | uint16(c.E) }
func (c *CPU) AF() uint16 { return uint16(c.A)<<8 | uint16(c.F) }

func (c *CPU) SetHL(v uint16) { c.H, c.L = byte(v>>8), byte(v) }
func (c *CPU) SetBC(v uint16) { c.B, c.C = byte(v>>8), byte(v) }
func (c *CPU) SetDE(v uint16) { c.D, c.E = byte(v>>8), byte(v) }
func (c *CPU) SetAF(v uint16) { c.A, c.F = byte(v>>8), byte(v) }

func (c *CPU) push(bus Bus, v uint16) {
	c.wait(bus, 1)
	c.SP--
	c.writeAbs(bus, c.SP, byte(v>>8), 3)
	c.SP--
	c.writeAbs(bus, c.SP, byte(v), 3)
}

func (c *CPU) pop(bus Bus) uint16 {
	lo := c.readAbs(bus, c.SP, 3)
	c.SP++
	hi := c.readAbs(bus, c.SP, 3)
	c.SP++
	return uint16(lo) | uint16(hi)<<8
}

// ioCycles runs the four T-states of an IN or OUT.
// C:n applies one contention delay then n T-states. N:n is n T-states.
// A read is sampled on the last T-state. A write retires after that T-state
// so the new latch is visible to the next pixel, not the one under the OUT.
func (c *CPU) ioCycles(bus Bus, port uint16, write bool, val byte) byte {
	var got byte
	seen := 0
	for _, g := range ioGroups(port) {
		if g.contend {
			if d := bus.ContendIO(port, bus.T()); d != 0 {
				bus.Advance(d)
			}
		}
		for i := 0; i < g.n; i++ {
			seen++
			if seen == 4 && !write {
				got = bus.IORead(port)
			}
			bus.Advance(1)
		}
	}
	if write {
		bus.IOWrite(port, val)
	}
	return got
}

type ioGroup struct {
	contend bool
	n       int
}

func ioGroups(port uint16) []ioGroup {
	high := byte(port >> 8)
	hi := high >= 0x40 && high <= 0x7F
	ula := port&1 == 0
	switch {
	case !hi && ula:
		return []ioGroup{{false, 1}, {true, 3}}
	case !hi && !ula:
		return []ioGroup{{false, 4}}
	case hi && ula:
		return []ioGroup{{true, 1}, {true, 3}}
	default:
		return []ioGroup{{true, 1}, {true, 1}, {true, 1}, {true, 1}}
	}
}
