package machine

// 128K frame. 228 T-states per line and 311 lines. Contention uses the same
// 6,5,4,3,2,1,0,0 pattern as the 48K, starting at T-state 14,361.
const (
	t128Line  = 228
	t128Lines = 311
	t128Frame = 70908
	t128Cont  = 14361

	// sampleHz128 keeps a 128K frame emitting the same number of beeper
	// samples as a 48K frame, so the 48 kHz player stays in step.
	sampleHz128 = CPUHz * t128Frame / TStatesPerFrame
)

func (m *Machine) Is128() bool { return m.model128 }

func (m *Machine) frameLen() int {
	if m.model128 {
		return t128Frame
	}
	return TStatesPerFrame
}

func (m *Machine) lineLen() int {
	if m.model128 {
		return t128Line
	}
	return TStatesPerLine
}

func (m *Machine) readMem(addr uint16) byte {
	if !m.model128 {
		return m.mem.Read(addr)
	}
	switch addr >> 14 {
	case 0:
		return m.roms[m.romSel][addr]
	case 1:
		return m.banks[5][addr&0x3FFF]
	case 2:
		return m.banks[2][addr&0x3FFF]
	default:
		return m.banks[m.ramPage][addr&0x3FFF]
	}
}

func (m *Machine) writeMem(addr uint16, v byte) {
	if !m.model128 {
		m.mem.Write(addr, v)
		return
	}
	switch addr >> 14 {
	case 0:
		return
	case 1:
		m.banks[5][addr&0x3FFF] = v
	case 2:
		m.banks[2][addr&0x3FFF] = v
	default:
		m.banks[m.ramPage][addr&0x3FFF] = v
	}
}

// screenByte reads the display file. Bank 5 is the normal screen. Bit 3 of
// port 0x7FFD selects bank 7 instead. The offset is from 0x4000.
func (m *Machine) screenByte(off int) byte {
	if !m.model128 {
		return m.mem.RAM[off]
	}
	bank := 5
	if m.screen7 {
		bank = 7
	}
	return m.banks[bank][off]
}

// writePage applies one write to port 0x7FFD. Bit 5 locks later writes
// until reset. The lock takes effect after this byte is applied.
func (m *Machine) writePage(v byte) {
	if m.pageLock {
		return
	}
	m.page = v
	m.ramPage = int(v & 7)
	m.romSel = int((v >> 4) & 1)
	m.screen7 = v&0x08 != 0
	m.pageLock = v&0x20 != 0
}

func (m *Machine) addrContended(addr uint16) bool {
	if !m.model128 {
		return addr >= 0x4000 && addr <= 0x7FFF
	}
	switch addr >> 14 {
	case 1:
		return true
	case 3:
		return m.ramPage&1 != 0
	default:
		return false
	}
}

func contentDelay128(t int) int {
	if t < 0 {
		return 0
	}
	t %= t128Frame
	if t < t128Cont {
		return 0
	}
	rel := t - t128Cont
	if rel >= 192*t128Line {
		return 0
	}
	pos := rel % t128Line
	if pos >= 128 {
		return 0
	}
	return contentionPattern[pos%8]
}

// install128 places a snapshot into the eight RAM banks.
// A 48K image is mapped as banks 5, 2, and 0, with the 48K ROM paged and locked.
func (m *Machine) install128(s Snap) {
	m.pageLock = false
	if s.Model128 {
		m.banks = s.Banks
		m.writePage(s.Page)
		m.ay.load(s.AYSel, s.AYReg)
		return
	}
	var banks [8][16384]byte
	copy(banks[5][:], s.RAM[0:16384])
	copy(banks[2][:], s.RAM[16384:32768])
	copy(banks[0][:], s.RAM[32768:49152])
	m.banks = banks
	m.writePage(0x30)
	m.ay.reset()
}
