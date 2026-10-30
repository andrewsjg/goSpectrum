package machine

import (
	"fmt"
	"os"
)

// ReadROM loads a 16,384-byte 48K image or a 32,768-byte 128K image.
// Any other length or an unreadable path returns an error and no bytes.
func ReadROM(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("rom: %w", err)
	}
	if len(b) != 16384 && len(b) != 32768 {
		return nil, fmt.Errorf("rom: %s is %d bytes, want 16384 or 32768", path, len(b))
	}
	return b, nil
}

// Charset returns the 96 8×8 glyphs, space to ©, from a 48K ROM or from the
// 48K BASIC half of a 128K ROM. Any other length returns nil.
func Charset(rom []byte) []byte {
	base := 0x3D00
	switch len(rom) {
	case 16384:
	case 32768:
		base += 16384
	default:
		return nil
	}
	return rom[base : base+96*8]
}
