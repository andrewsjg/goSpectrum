package machine

// Frame geometry and the late-timing 48K contention model.
const (
	CPUHz           = 3500000
	TStatesPerLine  = 224
	LinesPerFrame   = 312
	TStatesPerFrame = 69888
	VisibleW        = 352
	VisibleH        = 296
	PaperX          = 48
	PaperY          = 48
	AudioHz         = 48000
	AudioAmp        = 12000
)

var contentionPattern = [8]int{6, 5, 4, 3, 2, 1, 0, 0}

func normT(t int) int {
	if t < 0 {
		return 0
	}
	return t % TStatesPerFrame
}

// Delay is the contended-memory wait for an access that begins at T-state t.
// Addresses outside 0x4000–0x7FFF wait 0. The pattern is 6,5,4,3,2,1,0,0 for
// 128 T-states from T-state 14,335, then 96 idle, for each of 192 paper lines.
func Delay(t int, addr uint16) int {
	if addr < 0x4000 || addr > 0x7FFF {
		return 0
	}
	return contentDelay(t)
}

// ContentDelay is the same pattern with no address test, for I/O C-cycles.
func ContentDelay(t int) int { return contentDelay(t) }

func contentDelay(t int) int {
	t = normT(t)
	const start = 14335
	if t < start {
		return 0
	}
	rel := t - start
	if rel >= 192*TStatesPerLine {
		return 0
	}
	pos := rel % TStatesPerLine
	if pos >= 128 {
		return 0
	}
	return contentionPattern[pos%8]
}

// PaperAddr is the bitmap address and bit (7 is the leftmost pixel) for paper
// coordinates x in 0..255 and y in 0..191.
func PaperAddr(x, y int) (uint16, int) {
	off := ((y & 0xC0) << 5) | ((y & 0x07) << 8) | ((y & 0x38) << 2) | (x >> 3)
	return 0x4000 + uint16(off), 7 - (x & 7)
}

// PaletteIndex maps an attribute colour and bright bit. Colour 0 is index 0
// in both bright states; other bright colours are colour+8.
func PaletteIndex(color, bright byte) byte {
	color &= 7
	if color == 0 {
		return 0
	}
	if bright != 0 {
		return color + 8
	}
	return color
}

// RGB returns channel levels for a palette index. Blue is bit 0, red bit 1,
// green bit 2. Clear is 0x00, normal is 0xD7, bright is 0xFF.
func RGB(index byte) (r, g, b byte) {
	bright := index >= 8
	color := index & 7
	ch := func(bit byte) byte {
		if color&bit == 0 {
			return 0x00
		}
		if bright {
			return 0xFF
		}
		return 0xD7
	}
	return ch(0x02), ch(0x04), ch(0x01)
}

// FloatingBus is the byte the ULA is fetching at T-state t, or 0xFF when idle.
// The first bitmap byte (0x4000) is fetched at T-state 14,347.
func FloatingBus(t int, read func(uint16) byte) byte {
	t = normT(t)
	line := t / TStatesPerLine
	tin := t % TStatesPerLine
	if line < 64 || line >= 256 || tin < 11 || tin >= 11+128 {
		return 0xFF
	}
	pos := tin - 11
	sub := pos % 8
	if sub >= 4 {
		return 0xFF
	}
	col := (pos/8)*2 + sub/2
	y := line - 64
	switch sub {
	case 0, 2:
		addr, _ := PaperAddr(col*8, y)
		return read(addr)
	default:
		return read(uint16(0x5800 + (y>>3)*32 + col))
	}
}

// Frame is one 352×296 image of palette indexes. (0,0) is the top-left border.
// Paper pixel (0,0) is frame coordinate (48,48).
type Frame struct {
	Pix [VisibleW * VisibleH]byte
}

func (f Frame) At(x, y int) byte {
	if x < 0 || y < 0 || x >= VisibleW || y >= VisibleH {
		return 0
	}
	return f.Pix[y*VisibleW+x]
}

func (f Frame) Paper(x, y int) byte { return f.At(x+PaperX, y+PaperY) }
