package formats

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// ReadZ80 loads a v1, v2, or v3 file. Hardware mode 0 is a 48K.
// A v2 mode 3 or a v3 mode 4 is a 128K. PopPC is false; PC comes from the header.
func ReadZ80(b []byte) (Snapshot, error) {
	regs, border, compressed, v1, err := readZ80Header(b)
	if err != nil {
		return Snapshot{}, err
	}
	var ram [49152]byte
	if v1 {
		body := b[30:]
		if compressed {
			dec, err := decompress(body, true, 49152)
			if err != nil {
				return Snapshot{}, err
			}
			copy(ram[:], dec)
		} else {
			if len(body) != 49152 {
				return Snapshot{}, fmt.Errorf("corrupt Z80: v1 RAM is %d bytes", len(body))
			}
			copy(ram[:], body)
		}
		return Snapshot{Regs: regs, RAM: ram, Border: border}, nil
	}
	if len(b) < 32 {
		return Snapshot{}, fmt.Errorf("corrupt Z80: short v2 header")
	}
	extra := int(u16(b, 30))
	switch extra {
	case 23, 54, 55:
	default:
		return Snapshot{}, fmt.Errorf("corrupt Z80: extra header %d", extra)
	}
	if len(b) < 32+extra {
		return Snapshot{}, fmt.Errorf("corrupt Z80: truncated extra header")
	}
	regs.PC = u16(b, 32)
	is128 := false
	switch extra {
	case 23:
		is128 = b[34] == 3
		if b[34] != 0 && b[34] != 3 {
			return Snapshot{}, ErrBeyond48K
		}
	case 54, 55:
		is128 = b[34] == 4
		if b[34] != 0 && b[34] != 4 {
			return Snapshot{}, ErrBeyond48K
		}
	}
	off := 32 + extra
	if is128 {
		return readZ80_128(b, off, regs, border)
	}
	seen := map[byte]bool{}
	for off < len(b) {
		if len(b)-off < 3 {
			return Snapshot{}, fmt.Errorf("corrupt Z80: truncated page")
		}
		dlen := u16(b, off)
		page := b[off+2]
		off += 3
		var raw []byte
		if dlen == 0xFFFF {
			if len(b)-off < 16384 {
				return Snapshot{}, fmt.Errorf("corrupt Z80: short raw page %d", page)
			}
			raw = b[off : off+16384]
			off += 16384
		} else {
			n := int(dlen)
			if len(b)-off < n {
				return Snapshot{}, fmt.Errorf("corrupt Z80: short compressed page %d", page)
			}
			dec, err := decompress(b[off:off+n], false, 16384)
			if err != nil {
				return Snapshot{}, err
			}
			raw = dec
			off += n
		}
		if page != 4 && page != 5 && page != 8 {
			return Snapshot{}, ErrBeyond48K
		}
		base := map[byte]int{8: 0, 4: 16384, 5: 32768}[page]
		copy(ram[base:base+16384], raw)
		seen[page] = true
	}
	for _, p := range []byte{8, 4, 5} {
		if !seen[p] {
			return Snapshot{}, fmt.Errorf("corrupt Z80: missing page %d", p)
		}
	}
	return Snapshot{Regs: regs, RAM: ram, Border: border}, nil
}

// readZ80_128 maps pages 3..10 onto RAM banks 0..7.
// Byte 35 is the last write to port 0x7FFD. Bytes 38..54 are the AY.
func readZ80_128(b []byte, off int, regs Regs, border byte) (Snapshot, error) {
	var s Snapshot
	s.Regs = regs
	s.Border = border
	s.Machine128 = true
	if len(b) >= 55 {
		s.Page = b[35]
		s.AYSel = b[38]
		copy(s.AYReg[:], b[39:55])
	}
	seen := map[byte]bool{}
	for off < len(b) {
		raw, page, next, err := readZ80Page(b, off)
		if err != nil {
			return Snapshot{}, err
		}
		off = next
		if page < 3 || page > 10 {
			return Snapshot{}, ErrBeyond48K
		}
		bank := int(page - 3)
		copy(s.Banks[bank][:], raw)
		seen[page] = true
	}
	for page := byte(3); page <= 10; page++ {
		if !seen[page] {
			return Snapshot{}, fmt.Errorf("corrupt Z80: missing page %d", page)
		}
	}
	paged := int(s.Page & 7)
	copy(s.RAM[0:16384], s.Banks[5][:])
	copy(s.RAM[16384:32768], s.Banks[2][:])
	copy(s.RAM[32768:49152], s.Banks[paged][:])
	return s, nil
}

func readZ80Page(b []byte, off int) ([]byte, byte, int, error) {
	if len(b)-off < 3 {
		return nil, 0, 0, fmt.Errorf("corrupt Z80: truncated page")
	}
	dlen := u16(b, off)
	page := b[off+2]
	off += 3
	if dlen == 0xFFFF {
		if len(b)-off < 16384 {
			return nil, 0, 0, fmt.Errorf("corrupt Z80: short raw page %d", page)
		}
		return b[off : off+16384], page, off + 16384, nil
	}
	n := int(dlen)
	if len(b)-off < n {
		return nil, 0, 0, fmt.Errorf("corrupt Z80: short compressed page %d", page)
	}
	dec, err := decompress(b[off:off+n], false, 16384)
	if err != nil {
		return nil, 0, 0, err
	}
	return dec, page, off + n, nil
}

func readZ80Header(b []byte) (Regs, byte, bool, bool, error) {
	if len(b) < 30 {
		return Regs{}, 0, false, false, fmt.Errorf("corrupt Z80: short header")
	}
	flags := b[12]
	if flags == 255 {
		flags = 1
	}
	r := Regs{
		A:    b[0],
		F:    b[1],
		C:    b[2],
		B:    b[3],
		L:    b[4],
		H:    b[5],
		PC:   u16(b, 6),
		SP:   u16(b, 8),
		I:    b[10],
		R:    (b[11] & 0x7F) | ((flags & 1) << 7),
		E:    b[13],
		D:    b[14],
		Cp:   b[15],
		Bp:   b[16],
		Ep:   b[17],
		Dp:   b[18],
		Lp:   b[19],
		Hp:   b[20],
		Ap:   b[21],
		Fp:   b[22],
		IY:   u16(b, 23),
		IX:   u16(b, 25),
		IFF1: b[27] != 0,
		IFF2: b[28] != 0,
		IM:   b[29] & 3,
	}
	v1 := r.PC != 0
	if v1 && flags&0x10 != 0 {
		return Regs{}, 0, false, true, ErrBeyond48K
	}
	if b[29]&3 == 3 {
		return Regs{}, 0, false, v1, fmt.Errorf("corrupt Z80: IM 3")
	}
	return r, (flags >> 1) & 7, flags&0x20 != 0, v1, nil
}

// decompress expands ED ED <count> <byte> runs. withMarker stops before the
// four bytes 00 ED ED 00 and does not copy them. A count of 0 is corrupt.
func decompress(src []byte, withMarker bool, outLen int) ([]byte, error) {
	out := make([]byte, 0, outLen)
	for i := 0; i < len(src); {
		if withMarker && i+4 <= len(src) && src[i] == 0x00 && src[i+1] == 0xED && src[i+2] == 0xED && src[i+3] == 0x00 {
			if i+4 != len(src) {
				return nil, fmt.Errorf("corrupt Z80: data after end marker")
			}
			break
		}
		if i+1 < len(src) && src[i] == 0xED && src[i+1] == 0xED {
			if i+3 >= len(src) {
				return nil, fmt.Errorf("corrupt Z80: truncated run")
			}
			count := int(src[i+2])
			if count == 0 {
				return nil, fmt.Errorf("corrupt Z80: zero-length run")
			}
			if len(out)+count > outLen {
				return nil, fmt.Errorf("corrupt Z80: decompressed overrun")
			}
			by := src[i+3]
			for k := 0; k < count; k++ {
				out = append(out, by)
			}
			i += 4
			continue
		}
		if len(out) >= outLen {
			return nil, fmt.Errorf("corrupt Z80: decompressed overrun")
		}
		out = append(out, src[i])
		i++
	}
	if len(out) != outLen {
		return nil, fmt.Errorf("corrupt Z80: decompressed %d bytes, want %d", len(out), outLen)
	}
	return out, nil
}

// WriteZ80 emits a v3 48K file, hardware mode 0, with raw pages 8, 4, and 5.
func WriteZ80(s Snapshot) []byte {
	var hdr [86]byte
	hdr[0] = s.Regs.A
	hdr[1] = s.Regs.F
	binary.LittleEndian.PutUint16(hdr[2:], uint16(s.Regs.C)|uint16(s.Regs.B)<<8)
	binary.LittleEndian.PutUint16(hdr[4:], uint16(s.Regs.L)|uint16(s.Regs.H)<<8)
	// PC in the first header is 0 for v2/v3.
	binary.LittleEndian.PutUint16(hdr[8:], s.Regs.SP)
	hdr[10] = s.Regs.I
	hdr[11] = s.Regs.R & 0x7F
	flags := (s.Border & 7) << 1
	if s.Regs.R&0x80 != 0 {
		flags |= 1
	}
	hdr[12] = flags
	binary.LittleEndian.PutUint16(hdr[13:], uint16(s.Regs.E)|uint16(s.Regs.D)<<8)
	binary.LittleEndian.PutUint16(hdr[15:], uint16(s.Regs.Cp)|uint16(s.Regs.Bp)<<8)
	binary.LittleEndian.PutUint16(hdr[17:], uint16(s.Regs.Ep)|uint16(s.Regs.Dp)<<8)
	binary.LittleEndian.PutUint16(hdr[19:], uint16(s.Regs.Lp)|uint16(s.Regs.Hp)<<8)
	hdr[21] = s.Regs.Ap
	hdr[22] = s.Regs.Fp
	binary.LittleEndian.PutUint16(hdr[23:], s.Regs.IY)
	binary.LittleEndian.PutUint16(hdr[25:], s.Regs.IX)
	if s.Regs.IFF1 {
		hdr[27] = 1
	}
	if s.Regs.IFF2 {
		hdr[28] = 1
	}
	hdr[29] = s.Regs.IM & 3
	binary.LittleEndian.PutUint16(hdr[30:], 54)
	binary.LittleEndian.PutUint16(hdr[32:], s.Regs.PC)
	hdr[34] = 0
	var buf bytes.Buffer
	buf.Write(hdr[:])
	writeRawPage(&buf, 8, s.RAM[0:16384])
	writeRawPage(&buf, 4, s.RAM[16384:32768])
	writeRawPage(&buf, 5, s.RAM[32768:49152])
	return buf.Bytes()
}

func writeRawPage(buf *bytes.Buffer, page byte, data []byte) {
	var n [2]byte
	binary.LittleEndian.PutUint16(n[:], 0xFFFF)
	buf.Write(n[:])
	buf.WriteByte(page)
	buf.Write(data)
}
