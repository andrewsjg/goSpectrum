package formats

import "fmt"

type tzxBlock struct {
	id   byte
	data []byte

	pauseMs    int
	pilotLen   int
	sync1      int
	sync2      int
	zero       int
	one        int
	pilotCount int
	usedBits   int
	pulseLen   int
	count      int
	pulses     []int
	tPer       int
	delta      int
	level      int // 0x2B: 0 low, 1 high, -1 absent
	stop       bool
}

// ReadTZX parses a version-1 tape. A refused block id fails the whole file
// before any edges are returned. Major versions other than 1 are refused.
// A higher minor version is accepted.
func ReadTZX(b []byte) (Tape, error) {
	blocks, err := parseTZX(b)
	if err != nil {
		return Tape{}, err
	}
	return playTZX(blocks)
}

func parseTZX(b []byte) ([]tzxBlock, error) {
	if len(b) < 10 || string(b[:7]) != "ZXTape!" || b[7] != 0x1A {
		return nil, fmt.Errorf("corrupt TZX: bad header")
	}
	major, minor := int(b[8]), int(b[9])
	if major != 1 {
		return nil, fmt.Errorf("unsupported TZX version %d.%d", major, minor)
	}
	rest := b[10:]
	var blocks []tzxBlock
	for len(rest) > 0 {
		id := rest[0]
		rest = rest[1:]
		idx := len(blocks)
		bl, n, err := parseOne(id, rest, idx)
		if err != nil {
			return nil, err
		}
		bl.id = id
		blocks = append(blocks, bl)
		rest = rest[n:]
	}
	return blocks, nil
}

func unsupported(id byte, idx int) error {
	return fmt.Errorf("unsupported TZX block 0x%02X at block %d", id, idx)
}

func need(b []byte, n, idx int, id byte) error {
	if len(b) < n {
		return fmt.Errorf("corrupt TZX: truncated block 0x%02X at block %d", id, idx)
	}
	return nil
}

func parseOne(id byte, b []byte, idx int) (tzxBlock, int, error) {
	switch id {
	case 0x18, 0x19, 0x26, 0x27, 0x28:
		return tzxBlock{}, 0, unsupported(id, idx)
	case 0x10:
		if err := need(b, 4, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		pause := int(u16(b, 0))
		n := int(u16(b, 2))
		if err := need(b, 4+n, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		return tzxBlock{pauseMs: pause, data: append([]byte(nil), b[4:4+n]...)}, 4 + n, nil
	case 0x11:
		if err := need(b, 18, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		n := u24(b[15:])
		if err := need(b, 18+n, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		return tzxBlock{
			pilotLen:   int(u16(b, 0)),
			sync1:      int(u16(b, 2)),
			sync2:      int(u16(b, 4)),
			zero:       int(u16(b, 6)),
			one:        int(u16(b, 8)),
			pilotCount: int(u16(b, 10)),
			usedBits:   int(b[12]),
			pauseMs:    int(u16(b, 13)),
			data:       append([]byte(nil), b[18:18+n]...),
		}, 18 + n, nil
	case 0x12:
		if err := need(b, 4, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		return tzxBlock{pulseLen: int(u16(b, 0)), count: int(u16(b, 2))}, 4, nil
	case 0x13:
		if err := need(b, 1, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		n := int(b[0])
		if err := need(b, 1+2*n, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		pulses := make([]int, n)
		for i := 0; i < n; i++ {
			pulses[i] = int(u16(b, 1+2*i))
		}
		return tzxBlock{pulses: pulses}, 1 + 2*n, nil
	case 0x14:
		if err := need(b, 10, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		n := u24(b[7:])
		if err := need(b, 10+n, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		return tzxBlock{
			zero:     int(u16(b, 0)),
			one:      int(u16(b, 2)),
			usedBits: int(b[4]),
			pauseMs:  int(u16(b, 5)),
			data:     append([]byte(nil), b[10:10+n]...),
		}, 10 + n, nil
	case 0x15:
		if err := need(b, 8, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		n := u24(b[5:])
		if err := need(b, 8+n, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		return tzxBlock{
			tPer:     int(u16(b, 0)),
			pauseMs:  int(u16(b, 2)),
			usedBits: int(b[4]),
			data:     append([]byte(nil), b[8:8+n]...),
		}, 8 + n, nil
	case 0x20:
		if err := need(b, 2, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		return tzxBlock{pauseMs: int(u16(b, 0))}, 2, nil
	case 0x21:
		return skipN8(b, 1, idx, id)
	case 0x22:
		return tzxBlock{}, 0, nil
	case 0x23:
		if err := need(b, 2, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		return tzxBlock{delta: int(int16(u16(b, 0)))}, 2, nil
	case 0x24:
		if err := need(b, 2, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		return tzxBlock{count: int(u16(b, 0))}, 2, nil
	case 0x25:
		return tzxBlock{}, 0, nil
	case 0x2A:
		if err := need(b, 4, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		n := u32(b)
		if len(b) < 4 || n > len(b)-4 {
			return tzxBlock{}, 0, fmt.Errorf("corrupt TZX: truncated block 0x%02X at block %d", id, idx)
		}
		return tzxBlock{stop: true}, 4 + n, nil
	case 0x2B:
		if err := need(b, 4, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		n := u32(b)
		if len(b) < 4 || n > len(b)-4 {
			return tzxBlock{}, 0, fmt.Errorf("corrupt TZX: truncated block 0x%02X at block %d", id, idx)
		}
		lv := -1
		if n >= 1 {
			lv = int(b[4])
		}
		return tzxBlock{level: lv}, 4 + n, nil
	case 0x30:
		return skipN8(b, 1, idx, id)
	case 0x31:
		if err := need(b, 2, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		n := int(b[1])
		if err := need(b, 2+n, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		return tzxBlock{}, 2 + n, nil
	case 0x32:
		if err := need(b, 2, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		n := int(u16(b, 0))
		if err := need(b, 2+n, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		return tzxBlock{}, 2 + n, nil
	case 0x33:
		if err := need(b, 1, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		n := int(b[0])
		if err := need(b, 1+3*n, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		return tzxBlock{}, 1 + 3*n, nil
	case 0x35:
		if err := need(b, 20, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		n := u32(b[16:])
		if len(b) < 20 || n > len(b)-20 {
			return tzxBlock{}, 0, fmt.Errorf("corrupt TZX: truncated block 0x%02X at block %d", id, idx)
		}
		return tzxBlock{}, 20 + n, nil
	case 0x5A:
		if err := need(b, 9, idx, id); err != nil {
			return tzxBlock{}, 0, err
		}
		return tzxBlock{}, 9, nil
	default:
		return tzxBlock{}, 0, unsupported(id, idx)
	}
}

func skipN8(b []byte, at, idx int, id byte) (tzxBlock, int, error) {
	if err := need(b, at, idx, id); err != nil {
		return tzxBlock{}, 0, err
	}
	n := int(b[at-1])
	if err := need(b, at+n, idx, id); err != nil {
		return tzxBlock{}, 0, err
	}
	return tzxBlock{}, at + n, nil
}

type loopFrame struct {
	start int
	left  int
}

func playTZX(blocks []tzxBlock) (Tape, error) {
	var build builder
	var stack []loopFrame
	const maxSteps = 1000000
	steps := 0
	i := 0
	for i < len(blocks) {
		steps++
		if steps > maxSteps {
			return Tape{}, fmt.Errorf("corrupt TZX: loop limit")
		}
		bl := blocks[i]
		switch bl.id {
		case 0x10:
			if len(bl.data) == 0 {
				return Tape{}, fmt.Errorf("corrupt TZX: empty block 0x10 at block %d", i)
			}
			build.romBlock(bl.data, bl.pauseMs)
			i++
		case 0x11:
			build.pilot(bl.pilotCount, bl.pilotLen)
			build.flip(bl.sync1)
			build.flip(bl.sync2)
			build.dataBits(bl.data, bl.usedBits, bl.zero, bl.one)
			build.dataPause(bl.pauseMs)
			i++
		case 0x12:
			for n := 0; n < bl.count; n++ {
				build.flip(bl.pulseLen)
			}
			i++
		case 0x13:
			for _, d := range bl.pulses {
				build.flip(d)
			}
			i++
		case 0x14:
			build.dataBits(bl.data, bl.usedBits, bl.zero, bl.one)
			build.dataPause(bl.pauseMs)
			i++
		case 0x15:
			build.directBits(bl.data, bl.usedBits, bl.tPer)
			build.dataPause(bl.pauseMs)
			i++
		case 0x20:
			if bl.pauseMs == 0 {
				return Tape{Edges: build.edges, ForceLow: true}, nil
			}
			build.hold(false, bl.pauseMs*tMillisecond)
			i++
		case 0x23:
			if bl.delta == 0 {
				return Tape{}, fmt.Errorf("unsupported TZX block 0x23 at block %d: delta 0", i)
			}
			next := i + bl.delta
			if next < 0 || next >= len(blocks) {
				return Tape{}, fmt.Errorf("corrupt TZX: jump out of range at block %d", i)
			}
			i = next
		case 0x24:
			if bl.count == 0 {
				n, err := skipLoop(blocks, i)
				if err != nil {
					return Tape{}, err
				}
				i = n
				continue
			}
			stack = append(stack, loopFrame{start: i + 1, left: bl.count})
			i++
		case 0x25:
			if len(stack) == 0 {
				return Tape{}, fmt.Errorf("corrupt TZX: loop end without start at block %d", i)
			}
			top := &stack[len(stack)-1]
			top.left--
			if top.left > 0 {
				i = top.start
				continue
			}
			stack = stack[:len(stack)-1]
			i++
		case 0x2A:
			return Tape{Edges: build.edges}, nil
		case 0x2B:
			if bl.level >= 0 {
				build.setLevel(bl.level != 0)
			}
			i++
		default:
			// Skipped ids were consumed at parse time and carry no edges.
			i++
		}
	}
	return Tape{Edges: build.edges}, nil
}

func skipLoop(blocks []tzxBlock, at int) (int, error) {
	depth := 1
	i := at + 1
	for i < len(blocks) && depth > 0 {
		switch blocks[i].id {
		case 0x24:
			depth++
		case 0x25:
			depth--
			if depth == 0 {
				return i + 1, nil
			}
		}
		i++
	}
	return 0, fmt.Errorf("corrupt TZX: unclosed loop at block %d", at)
}
