package machine

// Memory is the 48K map: 16 KB ROM at 0x0000 and 48 KB RAM at 0x4000.
// Writes to ROM are ignored. There is no paging.
type Memory struct {
	ROM [16384]byte
	RAM [49152]byte
}

func (m *Memory) Read(addr uint16) byte {
	if addr < 0x4000 {
		return m.ROM[addr]
	}
	return m.RAM[addr-0x4000]
}

func (m *Memory) Write(addr uint16, v byte) {
	if addr < 0x4000 {
		return
	}
	m.RAM[addr-0x4000] = v
}
