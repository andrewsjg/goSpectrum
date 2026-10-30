package ui

import "testing"

func TestAtlasPixels(t *testing.T) {
	cs := make([]byte, 96*8)
	cs[('A'-32)*8+2] = 0x81 // leftmost and rightmost pixel on row 2 of 'A'
	buf := atlasPixels(cs)
	at := func(x, y int) byte { return buf[(y*96*8+x)*4+3] }
	gx := int('A'-32) * 8
	if at(gx, 2) != 0xFF || at(gx+7, 2) != 0xFF {
		t.Fatal("set bits are not opaque")
	}
	if at(gx+1, 2) != 0 || at(gx, 1) != 0 || at(gx-1, 2) != 0 {
		t.Fatal("clear bits are not transparent")
	}
	if NewFont(cs[:10]).charset != nil {
		t.Fatal("short charset was accepted")
	}
}
