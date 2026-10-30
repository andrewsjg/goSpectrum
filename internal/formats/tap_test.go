package formats

import (
	"bytes"
	"testing"
)

func withSum(b []byte) []byte {
	sum := byte(0)
	for _, v := range b {
		sum ^= v
	}
	return append(append([]byte{}, b...), sum)
}

func TestTAPRoundTripAndHeader(t *testing.T) {
	payload := make([]byte, 18)
	payload[0] = 0x00
	payload[1] = 0
	copy(payload[2:12], []byte("HELLO     "))
	payload[12] = 0x00
	payload[13] = 0x01
	payload[14] = 10
	payload[16] = 20
	block := withSum(payload)
	raw := WriteTAP([][]byte{block})
	blocks, tape, err := ReadTAP(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 || !bytes.Equal(blocks[0], block) {
		t.Fatal("round trip")
	}
	h, ok := Header(blocks[0])
	if !ok || h.Type != 0 || h.Name != "HELLO     " || h.Length != 256 || h.Param1 != 10 || h.Param2 != 20 {
		t.Fatalf("header %+v %v", h, ok)
	}
	n := 0
	for _, e := range tape.Edges {
		if e.Dur != 2168 {
			break
		}
		n++
	}
	if n != 8063 {
		t.Fatalf("header pilots %d", n)
	}
	if tape.Edges[8063].Dur != 667 || tape.Edges[8064].Dur != 735 {
		t.Fatal("sync")
	}
}

func TestTAPChecksumAndTruncation(t *testing.T) {
	bad := WriteTAP([][]byte{{0x00, 0x00, 0x01}})
	if _, _, err := ReadTAP(bad); err == nil {
		t.Fatal("bad checksum")
	}
	short := []byte{0x01, 0x00, 0x00}
	if _, _, err := ReadTAP(short); err == nil {
		t.Fatal("length below 2")
	}
	trunc := []byte{0x0A, 0x00, 0x01, 0x02}
	if _, _, err := ReadTAP(trunc); err == nil {
		t.Fatal("truncated")
	}
}

func TestTAPDataPilotCount(t *testing.T) {
	block := withSum([]byte{0xFF, 0x00})
	_, tape, err := ReadTAP(WriteTAP([][]byte{block}))
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
	if n != 3223 {
		t.Fatalf("data pilots %d", n)
	}
}
