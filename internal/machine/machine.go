package machine

import (
	"fmt"

	"go.spectrum/internal/z80"
)

// Snap is a 48K register and RAM image. PopPC asks for one RETN after the
// RAM is installed (SNA). The speaker latch is not part of the image.
type Snap struct {
	A, F       byte
	B, C       byte
	D, E       byte
	H, L       byte
	Ap, Fp     byte
	Bp, Cp     byte
	Dp, Ep     byte
	Hp, Lp     byte
	IX, IY     uint16
	SP, PC     uint16
	I, R       byte
	IFF1, IFF2 bool
	IM         byte
	Halted     bool
	MEMPTR     uint16
	Border     byte
	RAM        [49152]byte
	PopPC      bool
	Model128   bool
	Banks      [8][16384]byte
	Page       byte
	AYSel      byte
	AYReg      [16]byte
}

// Machine is one Spectrum: CPU, memory, ULA, keyboard, tape, and beeper.
// A 16 KB ROM is a 48K. A 32 KB ROM is a 128K, with port 0x7FFD paging and
// an AY-3-8912.
type Machine struct {
	cpu    z80.CPU
	mem    Memory
	keys   Keyboard
	host   [8]byte
	border byte
	mic    bool
	beep   Beeper

	model128 bool
	roms     [2][16384]byte
	banks    [8][16384]byte
	page     byte
	ramPage  int
	romSel   int
	screen7  bool
	pageLock bool
	ay       ayChip

	frameT     int
	frameIndex int
	framesRun  int
	pix        [VisibleW * VisibleH]byte
	next       [VisibleW * VisibleH]byte
	last       Frame
	samples    []int16
	pending    []int16

	turbo     bool
	paused    bool
	status    string
	errorText string
	tape      Player
	tapeHold  bool
	autoload  bool
	joy       byte
}

func New() *Machine {
	m := &Machine{status: "running"}
	m.cpu.Reset()
	m.samples = make([]int16, 0, 2048)
	m.pending = make([]int16, 0, 64)
	return m
}

func (m *Machine) CPU() *z80.CPU     { return &m.cpu }
func (m *Machine) Mem() *Memory      { return &m.mem }
func (m *Machine) FrameT() int       { return m.frameT }
func (m *Machine) Border() byte      { return m.border }
func (m *Machine) Speaker() bool     { return m.beep.Speaker }
func (m *Machine) Status() string    { return m.status }
func (m *Machine) ErrorText() string { return m.errorText }
func (m *Machine) Paused() bool      { return m.paused }

// TapePlaying reports that edges are being consumed. A cued tape is armed
// but held, so the pacer stays in real time until LOAD is submitted.
func (m *Machine) TapePlaying() bool { return m.tape.Playing() && !m.tapeHold }

func (m *Machine) SetHostKeys(rows [8]byte) { m.host = rows }

// Kempston joystick bits, active high, as read from port 0x1F.
const (
	JoyRight byte = 1 << iota
	JoyLeft
	JoyDown
	JoyUp
	JoyFire
)

// SetJoystick sets the Kempston port. Any odd port with A5 low reads it.
func (m *Machine) SetJoystick(bits byte) { m.joy = bits & 0x1F }

// TapeProgress is the played fraction of the armed tape, 0 to 1.
func (m *Machine) TapeProgress() float64 { return m.tape.Progress() }

// Charset is the 8×8 character set of the 48K BASIC ROM in use.
func (m *Machine) Charset() []byte {
	if m.model128 {
		return Charset(m.roms[1][:])
	}
	return Charset(m.mem.ROM[:])
}

// Snapshot captures registers, RAM, the border, and on a 128K the banks,
// the 0x7FFD latch, and the AY. Snapshot files have no halted flag, so a
// halted CPU is saved with PC on its HALT, which runs again on restore.
func (m *Machine) Snapshot() Snap {
	c := &m.cpu
	s := Snap{
		A: c.A, F: c.F, B: c.B, C: c.C, D: c.D, E: c.E, H: c.H, L: c.L,
		Ap: c.Ap, Fp: c.Fp, Bp: c.Bp, Cp: c.Cp, Dp: c.Dp, Ep: c.Ep, Hp: c.Hp, Lp: c.Lp,
		IX: c.IX, IY: c.IY, SP: c.SP, PC: c.PC, I: c.I, R: c.R,
		IFF1: c.IFF1, IFF2: c.IFF2, IM: c.IM,
		MEMPTR: c.MEMPTR,
		Border: m.border,
	}
	if c.Halted {
		s.PC--
	}
	if !m.model128 {
		s.RAM = m.mem.RAM
		return s
	}
	s.Model128 = true
	s.Banks = m.banks
	s.Page = m.page
	s.AYSel = byte(m.ay.sel)
	s.AYReg = m.ay.reg
	return s
}

func (m *Machine) SetError(text string) {
	m.errorText = text
	if !m.paused {
		m.status = "error"
	}
}

func (m *Machine) SetPaused(p bool) {
	m.paused = p
	if p {
		m.status = "paused"
		return
	}
	if m.errorText != "" {
		m.status = "error"
		return
	}
	m.status = "running"
}

func (m *Machine) TogglePause() { m.SetPaused(!m.paused) }

func (m *Machine) SetTurbo(on bool) {
	if m.turbo && !on {
		m.beep.Acc = 0
		m.pending = m.pending[:0]
	}
	m.turbo = on
}

// Reset is power-on. RAM is 0x00, the border is black, the speaker is clear,
// PC is 0, and frameT is 0. The ROM image is left in place.
func (m *Machine) Reset() {
	rom := m.mem.ROM
	model := m.model128
	roms := m.roms
	samples, pending := m.samples, m.pending
	if samples == nil {
		samples = make([]int16, 0, 2048)
	}
	if pending == nil {
		pending = make([]int16, 0, 64)
	}
	*m = Machine{
		mem:     Memory{ROM: rom},
		status:  "running",
		samples: samples[:0],
		pending: pending[:0],
	}
	m.cpu.Reset()
	if model {
		m.model128 = true
		m.roms = roms
		m.beep.Hz = sampleHz128
		m.ay.reset()
	}
}

func (m *Machine) LoadROM(rom []byte) error {
	switch len(rom) {
	case 16384:
		m.model128 = false
		copy(m.mem.ROM[:], rom)
	case 32768:
		m.model128 = true
		copy(m.roms[0][:], rom[:16384])
		copy(m.roms[1][:], rom[16384:])
		copy(m.mem.ROM[:], rom[:16384])
		m.beep.Hz = sampleHz128
		m.ay.reset()
	default:
		return fmt.Errorf("rom: %d bytes, want 16384 or 32768", len(rom))
	}
	return nil
}

// ArmTape cues an edge player. autoload types LOAD "" after 150 frames.
// Edges stay put until that Enter, or until the user presses Enter when
// autoload is off. The ROM's leader search burns about a second of pilot
// after the first edge; playing the leader during copyright loses it.
func (m *Machine) ArmTape(edges []Pulse, forceLow bool, autoload bool) {
	m.tape.Arm(edges, forceLow)
	m.autoload = autoload
	m.tapeHold = m.tape.Playing()
}

// ApplySnapshot replaces RAM, registers, and the border. frameT and the
// speaker latch are cleared. PopPC performs RETN exactly once.
func (m *Machine) ApplySnapshot(s Snap) {
	if s.Model128 && !m.model128 {
		m.SetError("this snapshot needs the 128K ROM")
		return
	}
	if m.model128 {
		m.install128(s)
	} else {
		m.mem.RAM = s.RAM
	}
	m.cpu.A, m.cpu.F = s.A, s.F
	m.cpu.B, m.cpu.C = s.B, s.C
	m.cpu.D, m.cpu.E = s.D, s.E
	m.cpu.H, m.cpu.L = s.H, s.L
	m.cpu.Ap, m.cpu.Fp = s.Ap, s.Fp
	m.cpu.Bp, m.cpu.Cp = s.Bp, s.Cp
	m.cpu.Dp, m.cpu.Ep = s.Dp, s.Ep
	m.cpu.Hp, m.cpu.Lp = s.Hp, s.Lp
	m.cpu.IX, m.cpu.IY = s.IX, s.IY
	m.cpu.SP, m.cpu.PC = s.SP, s.PC
	m.cpu.I, m.cpu.R = s.I, s.R
	m.cpu.IFF1, m.cpu.IFF2 = s.IFF1, s.IFF2
	m.cpu.IM = s.IM
	m.cpu.Halted = s.Halted
	m.cpu.MEMPTR = s.MEMPTR
	m.border = s.Border & 7
	m.mic = false
	m.beep = Beeper{}
	if m.model128 {
		m.beep.Hz = sampleHz128
	}
	m.frameT = 0
	m.frameIndex = 0
	m.pix = [VisibleW * VisibleH]byte{}
	m.next = [VisibleW * VisibleH]byte{}
	if s.PopPC {
		m.retnOnce()
	}
	if m.errorText != "" && !m.paused {
		m.status = "error"
	} else if !m.paused {
		m.status = "running"
	}
}

func (m *Machine) retnOnce() {
	sp := m.cpu.SP
	lo := m.readMem(sp)
	hi := m.readMem(sp + 1)
	m.cpu.SP = sp + 2
	m.cpu.PC = uint16(lo) | uint16(hi)<<8
	m.cpu.IFF1 = m.cpu.IFF2
	m.cpu.MEMPTR = m.cpu.PC
}

// RunFrame executes one 69,888 T-state frame, unless the machine is paused.
// Paused frames keep frameT, RAM, and registers, and emit no samples.
func (m *Machine) RunFrame() (Frame, []int16) {
	if m.paused {
		m.installKeys()
		return m.last, nil
	}
	m.installKeys()
	frameLen := m.frameLen()
	if m.frameT >= frameLen {
		m.frameT -= frameLen
		m.pix = m.next
		m.next = [VisibleW * VisibleH]byte{}
		m.samples = append(m.samples[:0], m.pending...)
		m.pending = m.pending[:0]
	} else {
		m.pix = [VisibleW * VisibleH]byte{}
		m.samples = m.samples[:0]
	}
	for m.frameT < frameLen {
		m.cpu.Step(m)
	}
	var fr Frame
	fr.Pix = m.pix
	m.last = fr
	m.frameIndex++
	m.framesRun++
	if m.turbo {
		return fr, nil
	}
	out := make([]int16, len(m.samples))
	copy(out, m.samples)
	return fr, out
}

func (m *Machine) installKeys() {
	m.keys.Load(m.host)
	m.releaseTape()
	if !m.autoload {
		return
	}
	if m.model128 {
		m.autoload128()
		return
	}
	phase := m.framesRun - 150
	if phase < 0 || phase >= 64 {
		return
	}
	slot := phase / 8
	if slot%2 != 0 {
		return
	}
	switch slot / 2 {
	case 0:
		m.keys.Press("J")
	case 1, 2:
		m.keys.Press("SYMBOL")
		m.keys.Press("P")
	case 3:
		m.keys.Press("ENTER")
	}
}

// releaseTape starts the cassette once LOAD is submitted. Autoload's Enter
// is the frame that begins with framesRun == 198 (150 plus the six earlier
// 8-frame slots). A manual load starts when the host Enter key is down.
func (m *Machine) releaseTape() {
	if !m.tapeHold {
		return
	}
	if m.model128 {
		// The 128 menu's Enter is not the 48K loader. Hold the tape until
		// the 48K ROM is paged and LD-BYTES is running.
		if m.romSel == 1 && m.cpu.PC >= 0x0556 && m.cpu.PC < 0x0606 {
			m.tapeHold = false
		}
		return
	}
	if m.autoload {
		if m.framesRun >= 198 {
			m.tapeHold = false
		}
		return
	}
	if m.host[6]&0x01 != 0 {
		m.tapeHold = false
	}
}

// autoload128 presses Enter once, after the editor menu is on screen.
// Tape Loader is the first item, so that Enter starts the cassette loader.
func (m *Machine) autoload128() {
	const pressAt = 250
	d := m.framesRun - pressAt
	if d >= 0 && d < 8 {
		m.keys.Press("ENTER")
	}
}

func (m *Machine) paint(t int) {
	frame := TStatesPerFrame
	lineT := TStatesPerLine
	origin := 16
	paper0 := 64
	leftAt := 200
	if m.model128 {
		frame = t128Frame
		lineT = t128Line
		origin = 15
		paper0 = 63
		leftAt = 204
	}
	tt := t
	buf := &m.pix
	if t >= frame {
		tt = t - frame
		buf = &m.next
		if tt < 0 || tt >= frame {
			return
		}
	}
	line := tt / lineT
	tin := tt % lineT
	var x0, y int
	paper := false
	switch {
	case tin >= leftAt:
		y = line + 1 - origin
		x0 = (tin - leftAt) * 2
	case tin <= 127:
		y = line - origin
		x0 = 48 + tin*2
		paper = line >= paper0 && line < paper0+192
	case tin <= 151:
		y = line - origin
		x0 = 304 + (tin-128)*2
	default:
		return
	}
	if y < 0 || y >= VisibleH || x0 < 0 || x0+1 >= VisibleW {
		return
	}
	var c0, c1 byte
	if paper {
		c0 = m.paperAt(x0-48, y-48)
		c1 = m.paperAt(x0-47, y-48)
	} else {
		c0, c1 = m.border, m.border
	}
	i := y*VisibleW + x0
	buf[i] = c0
	buf[i+1] = c1
}

func (m *Machine) paperAt(x, y int) byte {
	addr, bit := PaperAddr(x, y)
	inkOn := m.screenByte(int(addr-0x4000))&(1<<uint(bit)) != 0
	attr := m.screenByte(0x1800 + (y>>3)*32 + (x >> 3))
	paper := (attr >> 3) & 7
	ink := attr & 7
	if attr&0x80 != 0 && m.frameIndex%32 >= 16 {
		ink, paper = paper, ink
	}
	color := paper
	if inkOn {
		color = ink
	}
	bright := byte(0)
	if attr&0x40 != 0 {
		bright = 1
	}
	return PaletteIndex(color, bright)
}

func (m *Machine) emitSample() {
	if m.model128 {
		m.ay.tick()
	}
	if m.turbo {
		return
	}
	s, ok := m.beep.tickOne()
	if !ok {
		return
	}
	if m.model128 {
		sum := int(s) + int(m.ay.level)
		if sum > 32767 {
			sum = 32767
		} else if sum < -32768 {
			sum = -32768
		}
		s = int16(sum)
	}
	if m.frameT >= m.frameLen() {
		m.pending = append(m.pending, s)
		return
	}
	m.samples = append(m.samples, s)
}

// Bus

func (m *Machine) T() int { return m.frameT }

func (m *Machine) Advance(n int) {
	for i := 0; i < n; i++ {
		m.paint(m.frameT)
		if !m.tapeHold {
			m.tape.Advance(1)
		}
		m.emitSample()
		m.frameT++
	}
}

func (m *Machine) MemRead(addr uint16) byte     { return m.readMem(addr) }
func (m *Machine) MemWrite(addr uint16, v byte) { m.writeMem(addr, v) }

func (m *Machine) IORead(port uint16) byte {
	if port&1 == 0 {
		return m.readULA(port)
	}
	if port&0x20 == 0 {
		return m.joy
	}
	if m.model128 {
		if port&0xC002 == 0xC000 {
			return m.ay.read()
		}
		return 0xFF
	}
	return FloatingBus(m.frameT, m.mem.Read)
}

func (m *Machine) IOWrite(port uint16, v byte) {
	if WriteULA(&m.border, &m.mic, &m.beep.Speaker, port, v) {
		return
	}
	if !m.model128 {
		return
	}
	switch {
	case port&0xC002 == 0xC000:
		m.ay.selectReg(v)
	case port&0xC002 == 0x8000:
		m.ay.write(v)
	case port&0x8002 == 0:
		m.writePage(v)
	}
}

func (m *Machine) ContendMem(addr uint16, t int) int {
	if !m.model128 {
		return Delay(t, addr)
	}
	if !m.addrContended(addr) {
		return 0
	}
	return contentDelay128(t)
}

func (m *Machine) ContendIO(_ uint16, t int) int {
	if m.model128 {
		return contentDelay128(t)
	}
	return ContentDelay(t)
}

func (m *Machine) InterruptAsserted() bool {
	return m.frameT%m.frameLen() < 32
}

func (m *Machine) InterruptAck() byte { return 0xFF }

func (m *Machine) readULA(port uint16) byte {
	v := byte(0xA0) | m.keys.Read(byte(port>>8))
	if !m.tapeHold && m.tape.Level() {
		v |= 0x40
	}
	return v
}
