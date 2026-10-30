package formats

import (
	"encoding/binary"
	"fmt"
)

const (
	tPilot       = 2168
	tSync1       = 667
	tSync2       = 735
	tZero        = 855
	tOne         = 1710
	nPilotShort  = 8063
	nPilotLong   = 3223
	tMillisecond = 3500
)

type builder struct {
	edges []Pulse
	level bool
}

func (b *builder) flip(dur int) {
	if dur <= 0 {
		return
	}
	b.level = !b.level
	b.edges = append(b.edges, Pulse{Level: b.level, Dur: dur})
}

func (b *builder) hold(level bool, dur int) {
	if dur <= 0 {
		return
	}
	b.level = level
	b.edges = append(b.edges, Pulse{Level: level, Dur: dur})
}

func (b *builder) setLevel(level bool) {
	b.level = level
	b.edges = append(b.edges, Pulse{Level: level, Dur: 0})
}

func (b *builder) pilot(n, length int) {
	for i := 0; i < n; i++ {
		b.flip(length)
	}
}

func (b *builder) dataBits(data []byte, used, zero, one int) {
	if len(data) == 0 {
		return
	}
	if used < 1 || used > 8 {
		used = 8
	}
	for i, by := range data {
		bits := 8
		if i == len(data)-1 {
			bits = used
		}
		for bit := 0; bit < bits; bit++ {
			dur := zero
			if by&(1<<uint(7-bit)) != 0 {
				dur = one
			}
			b.flip(dur)
			b.flip(dur)
		}
	}
}

func (b *builder) directBits(data []byte, used, tstates int) {
	if len(data) == 0 || tstates <= 0 {
		return
	}
	if used < 1 || used > 8 {
		used = 8
	}
	for i, by := range data {
		bits := 8
		if i == len(data)-1 {
			bits = used
		}
		for bit := 0; bit < bits; bit++ {
			b.hold(by&(1<<uint(7-bit)) != 0, tstates)
		}
	}
}

// dataPause holds the opposite of the last pulse for 1 ms, then low for the
// rest of the pause. A 1 ms pause is only the opposite hold. A pause of 0
// changes nothing. The two low stretches stay separate edges.
func (b *builder) dataPause(ms int) {
	if ms <= 0 {
		return
	}
	total := ms * tMillisecond
	opp := !b.level
	if total <= tMillisecond {
		b.hold(opp, total)
		return
	}
	b.hold(opp, tMillisecond)
	b.hold(false, total-tMillisecond)
}

func (b *builder) romBlock(data []byte, pauseMs int) {
	n := nPilotLong
	if len(data) > 0 && data[0] < 128 {
		n = nPilotShort
	}
	b.pilot(n, tPilot)
	b.flip(tSync1)
	b.flip(tSync2)
	b.dataBits(data, 8, tZero, tOne)
	b.dataPause(pauseMs)
}

func u16(b []byte, i int) uint16 {
	return binary.LittleEndian.Uint16(b[i:])
}

func u24(b []byte) int {
	return int(b[0]) | int(b[1])<<8 | int(b[2])<<16
}

func u32(b []byte) int {
	return int(binary.LittleEndian.Uint32(b))
}

// Header reports the fields of a 19-byte header block (flag 0x00).
func Header(block []byte) (TAPHeader, bool) {
	if len(block) != 19 || block[0] != 0x00 {
		return TAPHeader{}, false
	}
	return TAPHeader{
		Type:   block[1],
		Name:   string(block[2:12]),
		Length: u16(block, 12),
		Param1: u16(block, 14),
		Param2: u16(block, 16),
	}, true
}

// WriteTAP length-prefixes each block. A block already includes its flag and checksum.
func WriteTAP(blocks [][]byte) []byte {
	var out []byte
	for _, bl := range blocks {
		var n [2]byte
		binary.LittleEndian.PutUint16(n[:], uint16(len(bl)))
		out = append(out, n[:]...)
		out = append(out, bl...)
	}
	return out
}

// ReadTAP checks every block before returning edges. A bad checksum or a
// short length fails the whole file.
func ReadTAP(b []byte) ([][]byte, Tape, error) {
	var blocks [][]byte
	var build builder
	for len(b) > 0 {
		if len(b) < 2 {
			return nil, Tape{}, fmt.Errorf("corrupt TAP: truncated length")
		}
		n := int(u16(b, 0))
		b = b[2:]
		if n < 2 {
			return nil, Tape{}, fmt.Errorf("corrupt TAP: block length %d", n)
		}
		if len(b) < n {
			return nil, Tape{}, fmt.Errorf("corrupt TAP: truncated block")
		}
		bl := append([]byte(nil), b[:n]...)
		b = b[n:]
		sum := byte(0)
		for _, v := range bl[:n-1] {
			sum ^= v
		}
		if sum != bl[n-1] {
			return nil, Tape{}, fmt.Errorf("corrupt TAP: checksum")
		}
		blocks = append(blocks, bl)
		build.romBlock(bl, 1000)
	}
	return blocks, Tape{Edges: build.edges}, nil
}
