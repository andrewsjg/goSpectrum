package formats

import (
	"strings"
	"testing"
)

func tzxFile(blocks ...[]byte) []byte {
	b := []byte("ZXTape!\x1a\x01\x00")
	for _, bl := range blocks {
		b = append(b, bl...)
	}
	return b
}

func TestTZXVersionAndRefused(t *testing.T) {
	bad := []byte("ZXTape!\x1a\x02\x00")
	_, err := ReadTZX(bad)
	if err == nil || !strings.Contains(err.Error(), "unsupported TZX version 2.0") {
		t.Fatal(err)
	}
	_, err = ReadTZX(tzxFile([]byte{0x19}))
	if err == nil || !strings.Contains(err.Error(), "0x19") || !strings.Contains(err.Error(), "block 0") {
		t.Fatal(err)
	}
	_, err = ReadTZX(tzxFile([]byte{0x30, 0}, []byte{0x5A, 0, 0, 0, 0, 0, 0, 0, 0, 0}, []byte{0x19}))
	if err == nil || !strings.Contains(err.Error(), "0x19") || !strings.Contains(err.Error(), "block 2") {
		t.Fatal(err)
	}
	for _, id := range []byte{0x18, 0x26, 0x27, 0x28, 0x16} {
		_, err = ReadTZX(tzxFile([]byte{id}))
		if err == nil || !strings.Contains(err.Error(), "unsupported TZX block") {
			t.Fatalf("%02X: %v", id, err)
		}
	}
	_, err = ReadTZX(tzxFile([]byte{0x23, 0, 0}))
	if err == nil || !strings.Contains(err.Error(), "delta 0") {
		t.Fatal(err)
	}
	// A higher minor version is accepted.
	ok := []byte("ZXTape!\x1a\x01\x14")
	ok = append(ok, 0x12, 10, 0, 1, 0)
	if _, err := ReadTZX(ok); err != nil {
		t.Fatal(err)
	}
}

func TestTZXStandardAndSkip(t *testing.T) {
	// 0x10 of flag 0x00, one zero byte, checksum, pause 1000.
	data := withSum([]byte{0x00, 0x00})
	block := []byte{0x10, 0xE8, 0x03, byte(len(data)), 0}
	block = append(block, data...)
	tape, err := ReadTZX(tzxFile([]byte{0x30, 2, 'h', 'i'}, []byte{0x5A, 'X', 'T', 'a', 'p', 'e', '!', 0x1A, 1, 0}, block))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range tape.Edges {
		if e.Dur != 2168 {
			break
		}
		n++
	}
	if n != 8063 || !tape.Edges[0].Level {
		t.Fatalf("pilots %d level %v", n, tape.Edges[0].Level)
	}
	if tape.Edges[8063].Dur != 667 || tape.Edges[8064].Dur != 735 {
		t.Fatal("sync")
	}
}

func TestTZXPulsesLoopsAndDirect(t *testing.T) {
	tone, err := ReadTZX(tzxFile([]byte{0x12, 50, 0, 3, 0}))
	if err != nil {
		t.Fatal(err)
	}
	if len(tone.Edges) != 3 || tone.Edges[0].Dur != 50 || !tone.Edges[0].Level || tone.Edges[1].Level {
		t.Fatalf("tone %+v", tone.Edges)
	}
	seq, err := ReadTZX(tzxFile([]byte{0x13, 2, 100, 0, 200, 0}))
	if err != nil || len(seq.Edges) != 2 || seq.Edges[1].Dur != 200 {
		t.Fatalf("seq %+v %v", seq.Edges, err)
	}
	// Jump +2 skips the middle pulse.
	jump, err := ReadTZX(tzxFile(
		[]byte{0x23, 2, 0},
		[]byte{0x12, 10, 0, 1, 0},
		[]byte{0x12, 20, 0, 1, 0},
	))
	if err != nil || len(jump.Edges) != 1 || jump.Edges[0].Dur != 20 {
		t.Fatalf("jump %+v %v", jump.Edges, err)
	}
	loop, err := ReadTZX(tzxFile(
		[]byte{0x24, 3, 0},
		[]byte{0x12, 5, 0, 1, 0},
		[]byte{0x25},
	))
	if err != nil || len(loop.Edges) != 3 {
		t.Fatalf("loop %+v %v", loop.Edges, err)
	}
	skip, err := ReadTZX(tzxFile(
		[]byte{0x24, 0, 0},
		[]byte{0x12, 5, 0, 1, 0},
		[]byte{0x25},
		[]byte{0x12, 7, 0, 1, 0},
	))
	if err != nil || len(skip.Edges) != 1 || skip.Edges[0].Dur != 7 {
		t.Fatalf("skip %+v %v", skip.Edges, err)
	}
	nested, err := ReadTZX(tzxFile(
		[]byte{0x24, 2, 0},
		[]byte{0x24, 2, 0},
		[]byte{0x12, 1, 0, 1, 0},
		[]byte{0x25},
		[]byte{0x25},
	))
	if err != nil || len(nested.Edges) != 4 {
		t.Fatalf("nested %+v %v", nested.Edges, err)
	}
	// 0x11 custom timings, one byte 0x80, pause 0.
	body := []byte{0x11,
		100, 0, 10, 0, 20, 0, 30, 0, 40, 0, 2, 0,
		8, 0, 0, 1, 0, 0, 0x80,
	}
	custom, err := ReadTZX(tzxFile(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(custom.Edges) != 20 || custom.Edges[0].Dur != 100 || custom.Edges[2].Dur != 10 || custom.Edges[4].Dur != 40 || custom.Edges[6].Dur != 30 {
		t.Fatalf("0x11 %+v", custom.Edges[:8])
	}
	direct, err := ReadTZX(tzxFile([]byte{0x15, 50, 0, 0, 0, 8, 1, 0, 0, 0x80}))
	if err != nil {
		t.Fatal(err)
	}
	if len(direct.Edges) != 8 || !direct.Edges[0].Level || direct.Edges[0].Dur != 50 || direct.Edges[1].Level {
		t.Fatalf("direct %+v", direct.Edges)
	}
	stop, err := ReadTZX(tzxFile([]byte{0x12, 9, 0, 1, 0}, []byte{0x2A, 0, 0, 0, 0}, []byte{0x12, 8, 0, 1, 0}))
	if err != nil || len(stop.Edges) != 1 {
		t.Fatalf("2A %+v %v", stop.Edges, err)
	}
}
