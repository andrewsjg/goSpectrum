package machine

// Keyboard is the 8×5 matrix. Bits set in down mean the key is held.
// Ghosting is applied when the row is read, not when the key is stored.
type Keyboard struct {
	down [8]byte
}

var keyPos = map[string][2]int{
	"CAPS": {0, 0}, "Z": {0, 1}, "X": {0, 2}, "C": {0, 3}, "V": {0, 4},
	"A": {1, 0}, "S": {1, 1}, "D": {1, 2}, "F": {1, 3}, "G": {1, 4},
	"Q": {2, 0}, "W": {2, 1}, "E": {2, 2}, "R": {2, 3}, "T": {2, 4},
	"1": {3, 0}, "2": {3, 1}, "3": {3, 2}, "4": {3, 3}, "5": {3, 4},
	"0": {4, 0}, "9": {4, 1}, "8": {4, 2}, "7": {4, 3}, "6": {4, 4},
	"P": {5, 0}, "O": {5, 1}, "I": {5, 2}, "U": {5, 3}, "Y": {5, 4},
	"ENTER": {6, 0}, "L": {6, 1}, "K": {6, 2}, "J": {6, 3}, "H": {6, 4},
	"SPACE": {7, 0}, "SYMBOL": {7, 1}, "M": {7, 2}, "N": {7, 3}, "B": {7, 4},
}

func (k *Keyboard) Clear() { k.down = [8]byte{} }

func (k *Keyboard) Set(row, col int, down bool) {
	if row < 0 || row > 7 || col < 0 || col > 4 {
		return
	}
	bit := byte(1 << uint(col))
	if down {
		k.down[row] |= bit
	} else {
		k.down[row] &^= bit
	}
}

func (k *Keyboard) Press(name string) {
	if p, ok := keyPos[name]; ok {
		k.Set(p[0], p[1], true)
	}
}

func (k *Keyboard) Load(rows [8]byte) { k.down = rows }

// Rows is a copy of the raw matrix, before the read-time ghosting mix.
func (k Keyboard) Rows() [8]byte { return k.down }

// Read returns the low five bits for a port whose high byte is rowSel.
// A bit is 0 when that column is pressed on a selected row (or ghosted).
func (k *Keyboard) Read(rowSel byte) byte {
	low := byte(0x1F)
	for col := 0; col < 5; col++ {
		if k.column(rowSel, col) {
			low &^= 1 << uint(col)
		}
	}
	return low
}

func (k *Keyboard) held(row, col int) bool {
	return k.down[row]&(1<<uint(col)) != 0
}

func (k *Keyboard) column(rowSel byte, col int) bool {
	for row := 0; row < 8; row++ {
		if rowSel&(1<<uint(row)) != 0 {
			continue
		}
		if k.held(row, col) {
			return true
		}
		for c2 := 0; c2 < 5; c2++ {
			if c2 == col || !k.held(row, c2) {
				continue
			}
			for r2 := 0; r2 < 8; r2++ {
				if k.held(r2, c2) && k.held(r2, col) {
					return true
				}
			}
		}
	}
	return false
}
