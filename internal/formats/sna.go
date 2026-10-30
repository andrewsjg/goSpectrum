package formats

import (
	"encoding/binary"
	"fmt"
)

const (
	snaLen    = 49179
	sna128Len = 131103
)

// ReadSNA loads a 48K snapshot, or a 131,103-byte 128K snapshot.
// A 48K file has no PC. PopPC is set and SP is the header SP.
// Any other length above 49,179 bytes is a machine this emulator does not run.
func ReadSNA(b []byte) (Snapshot, error) {
	if len(b) != snaLen && len(b) != sna128Len {
		if len(b) > snaLen {
			return Snapshot{}, ErrBeyond48K
		}
		return Snapshot{}, fmt.Errorf("corrupt SNA: %d bytes", len(b))
	}
	s, err := readSNARegs(b)
	if err != nil {
		return Snapshot{}, err
	}
	if len(b) == snaLen {
		s.PopPC = true
		copy(s.RAM[:], b[27:27+49152])
		return s, nil
	}
	return readSNA128(b, s)
}

func readSNARegs(b []byte) (Snapshot, error) {
	im := b[25]
	if im > 2 {
		return Snapshot{}, fmt.Errorf("corrupt SNA: IM %d", im)
	}
	var s Snapshot
	s.Border = b[26] & 7
	s.Regs = Regs{
		I:    b[0],
		Lp:   b[1],
		Hp:   b[2],
		Ep:   b[3],
		Dp:   b[4],
		Cp:   b[5],
		Bp:   b[6],
		Fp:   b[7],
		Ap:   b[8],
		L:    b[9],
		H:    b[10],
		E:    b[11],
		D:    b[12],
		C:    b[13],
		B:    b[14],
		IY:   u16(b, 15),
		IX:   u16(b, 17),
		IFF2: b[19]&0x04 != 0,
		R:    b[20],
		F:    b[21],
		A:    b[22],
		SP:   u16(b, 23),
		IM:   im,
	}
	s.Regs.IFF1 = s.Regs.IFF2
	return s, nil
}

// readSNA128 reads the 128K trailer. The 48K body is banks 5, 2, and the
// bank paged at 0xC000. The PC is in the trailer, so PopPC stays clear.
// The five following banks are the rest, in ascending order.
func readSNA128(b []byte, s Snapshot) (Snapshot, error) {
	s.Machine128 = true
	s.Regs.PC = u16(b, 49179)
	s.Page = b[49181]
	if b[49182] != 0 {
		return Snapshot{}, fmt.Errorf("SNA: TR-DOS paging is not emulated")
	}
	paged := int(s.Page & 7)
	if paged == 2 || paged == 5 {
		return Snapshot{}, fmt.Errorf("corrupt SNA: 128K file pages bank %d", paged)
	}
	copy(s.Banks[5][:], b[27:27+16384])
	copy(s.Banks[2][:], b[27+16384:27+32768])
	copy(s.Banks[paged][:], b[27+32768:27+49152])
	off := 49183
	for bank := 0; bank < 8; bank++ {
		if bank == 5 || bank == 2 || bank == paged {
			continue
		}
		copy(s.Banks[bank][:], b[off:off+16384])
		off += 16384
	}
	copy(s.RAM[0:16384], s.Banks[5][:])
	copy(s.RAM[16384:32768], s.Banks[2][:])
	copy(s.RAM[32768:49152], s.Banks[paged][:])
	return s, nil
}

// WriteSNA pushes PC into a copy of RAM and points the file SP at it.
// IFF2 is stored in bit 2 of byte 19. The writer is for tests, not a save UI.
func WriteSNA(s Snapshot) ([]byte, error) {
	ram := s.RAM
	sp := s.Regs.SP - 2
	if err := pokeRAM(&ram, sp, byte(s.Regs.PC)); err != nil {
		return nil, err
	}
	if err := pokeRAM(&ram, sp+1, byte(s.Regs.PC>>8)); err != nil {
		return nil, err
	}
	out := make([]byte, snaLen)
	out[0] = s.Regs.I
	binary.LittleEndian.PutUint16(out[1:], uint16(s.Regs.Lp)|uint16(s.Regs.Hp)<<8)
	binary.LittleEndian.PutUint16(out[3:], uint16(s.Regs.Ep)|uint16(s.Regs.Dp)<<8)
	binary.LittleEndian.PutUint16(out[5:], uint16(s.Regs.Cp)|uint16(s.Regs.Bp)<<8)
	out[7] = s.Regs.Fp
	out[8] = s.Regs.Ap
	binary.LittleEndian.PutUint16(out[9:], uint16(s.Regs.L)|uint16(s.Regs.H)<<8)
	binary.LittleEndian.PutUint16(out[11:], uint16(s.Regs.E)|uint16(s.Regs.D)<<8)
	binary.LittleEndian.PutUint16(out[13:], uint16(s.Regs.C)|uint16(s.Regs.B)<<8)
	binary.LittleEndian.PutUint16(out[15:], s.Regs.IY)
	binary.LittleEndian.PutUint16(out[17:], s.Regs.IX)
	if s.Regs.IFF2 {
		out[19] = 0x04
	}
	out[20] = s.Regs.R
	out[21] = s.Regs.F
	out[22] = s.Regs.A
	binary.LittleEndian.PutUint16(out[23:], sp)
	out[25] = s.Regs.IM & 3
	out[26] = s.Border & 7
	copy(out[27:], ram[:])
	return out, nil
}

func pokeRAM(ram *[49152]byte, addr uint16, v byte) error {
	if addr < 0x4000 {
		return fmt.Errorf("SNA writer: address %04X is in ROM", addr)
	}
	ram[addr-0x4000] = v
	return nil
}
