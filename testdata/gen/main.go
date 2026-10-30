// Command gen writes the headless ROMs used by go test.
// Run it from the module root: go run ./testdata/gen
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	root := repoRoot()
	if err := write(filepath.Join(root, "testdata", "test.rom"), testROM()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := write(filepath.Join(root, "testdata", "display.rom"), displayROM()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func write(path string, b []byte) error {
	if len(b) != 16384 {
		return fmt.Errorf("%s: %d bytes", path, len(b))
	}
	return os.WriteFile(path, b, 0o644)
}

func repoRoot() string {
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		p := filepath.Dir(d)
		if p == d {
			return "."
		}
		d = p
	}
}

// testROM stores 0xFF at 0x4000, attribute 0x07 at 0x5800, border 1, then JR $.
func testROM() []byte {
	rom := make([]byte, 16384)
	copy(rom, []byte{
		0x3E, 0xFF, 0x32, 0x00, 0x40,
		0x3E, 0x07, 0x32, 0x00, 0x58,
		0x3E, 0x01, 0xD3, 0xFE,
		0x18, 0xFE,
	})
	return rom
}

// displayROM fills cell 0 with bright yellow ink, cell 1 with yellow ink,
// then alternates border colours 1 and 2.
func displayROM() []byte {
	rom := make([]byte, 16384)
	p := 0
	emit := func(b ...byte) {
		copy(rom[p:], b)
		p += len(b)
	}
	djnz := func(target int) {
		rom[p] = 0x10
		rom[p+1] = byte(target - (p + 2))
		p += 2
	}
	jr := func(target int) {
		rom[p] = 0x18
		rom[p+1] = byte(target - (p + 2))
		p += 2
	}
	emit(0x21, 0x00, 0x40) // LD HL,0x4000
	emit(0x3E, 0xFF)       // LD A,0xFF
	emit(0x06, 0x08)       // LD B,8
	loop1 := p
	emit(0x77, 0x24) // LD (HL),A; INC H
	djnz(loop1)
	emit(0x3E, 0x46)       // bright yellow ink
	emit(0x32, 0x00, 0x58) // LD (0x5800),A
	emit(0x21, 0x01, 0x40) // LD HL,0x4001
	emit(0x3E, 0xFF)
	emit(0x06, 0x08)
	loop2 := p
	emit(0x77, 0x24)
	djnz(loop2)
	emit(0x3E, 0x06) // yellow ink
	emit(0x32, 0x01, 0x58)
	border := p
	emit(0x3E, 0x01, 0xD3, 0xFE)
	emit(0x3E, 0x02, 0xD3, 0xFE)
	jr(border)
	return rom
}
