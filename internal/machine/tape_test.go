package machine

import (
	"testing"

	"go.spectrum/internal/formats"
)

func TestTapePlayerTimings(t *testing.T) {
	block := []byte{0xFF, 0x80, 0xFF ^ 0x80}
	_, tape, err := formats.ReadTAP(formats.WriteTAP([][]byte{block}))
	if err != nil {
		t.Fatal(err)
	}
	e := tape.Edges
	if len(e) < 3275 {
		t.Fatalf("edges %d", len(e))
	}
	if e[0].Dur != 2168 || !e[0].Level {
		t.Fatalf("pilot %+v", e[0])
	}
	if e[3222].Dur != 2168 || e[3223].Dur != 667 || e[3224].Dur != 735 {
		t.Fatalf("sync %d %d %d", e[3222].Dur, e[3223].Dur, e[3224].Dur)
	}
	// Flag 0xFF is eight one-bits. The following 0x80 starts with a one, then a zero.
	if e[3225].Dur != 1710 || e[3243].Dur != 855 {
		t.Fatalf("bits %d %d", e[3225].Dur, e[3243].Dur)
	}
	pause := e[len(e)-2:]
	if pause[0].Dur != 3500 || pause[0].Level {
		t.Fatalf("opposite hold %+v", pause[0])
	}
	if pause[1].Dur != 3500000-3500 || pause[1].Level {
		t.Fatalf("rest low %+v", pause[1])
	}

	m := New()
	pulses := make([]Pulse, len(e))
	var total int
	for i, p := range e {
		pulses[i] = Pulse{Level: p.Level, Dur: p.Dur}
		total += p.Dur
	}
	m.ArmTape(pulses, false, false)
	m.tapeHold = false // this test checks pulse timing, not the LOAD cue
	if !m.TapePlaying() || m.IORead(0xFE)&0x40 == 0 {
		t.Fatal("tape should start high on bit 6")
	}
	m.Advance(2168)
	if m.IORead(0xFE)&0x40 != 0 {
		t.Fatal("second pilot pulse is low")
	}
	m.Advance(total)
	if m.TapePlaying() {
		t.Fatal("player still running")
	}
}

func TestTapeStopForcesLow(t *testing.T) {
	raw := []byte("ZXTape!\x1a\x01\x00")
	raw = append(raw, 0x12, 100, 0, 1, 0)
	raw = append(raw, 0x20, 0, 0)
	raw = append(raw, 0x12, 50, 0, 1, 0)
	tape, err := formats.ReadTZX(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !tape.ForceLow || len(tape.Edges) != 1 || tape.Edges[0].Dur != 100 {
		t.Fatalf("stop tape %+v", tape)
	}
	var p Player
	p.Arm([]Pulse{{Level: true, Dur: 100}}, true)
	p.Advance(100)
	if p.Playing() || p.Level() {
		t.Fatal("0x20 of 0 should stop and force low")
	}
	p.Arm([]Pulse{{Level: true, Dur: 10}}, false)
	p.Stop()
	if p.Playing() || p.Level() {
		t.Fatal("Stop")
	}
}
