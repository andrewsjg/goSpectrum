// Package formats reads and writes the published TAP, TZX, SNA, and Z80
// layouts. It does not import the machine or the UI.
package formats

import "errors"

// ErrBeyond48K is the refusal for a snapshot of a machine this emulator does
// not implement. The text is fixed by the spec.
var ErrBeyond48K = errors.New("the file targets a machine configuration beyond the emulated 48K")

// Pulse is one stretch of a constant EAR level, measured in T-states.
type Pulse struct {
	Level bool
	Dur   int
}

// Tape is the edge stream a file decoded to. ForceLow is set when a TZX
// pause of 0 stopped the tape; the player then idles low.
type Tape struct {
	Edges    []Pulse
	ForceLow bool
}

// Regs is the programmer-visible Z80 state stored by a snapshot.
// Multi-byte values are already host integers; the file endianness stays in
// the parser.
type Regs struct {
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
}

// Snapshot is a 48K or 128K image. RAM index 0 is address 0x4000. PopPC asks
// the machine to perform RETN once after the RAM is installed (48K SNA).
// Machine128 selects the eight 16 KB banks, the 0x7FFD latch, and the AY.
type Snapshot struct {
	Regs       Regs
	RAM        [49152]byte
	Border     byte
	PopPC      bool
	Machine128 bool
	Banks      [8][16384]byte
	Page       byte
	AYSel      byte
	AYReg      [16]byte
}

// TAPHeader is the 17-byte payload of a 19-byte flag-0x00 block.
type TAPHeader struct {
	Type   byte
	Name   string
	Length uint16
	Param1 uint16
	Param2 uint16
}
