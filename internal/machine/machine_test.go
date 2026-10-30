package machine

import "testing"

func jrROM() []byte {
	rom := make([]byte, 16384)
	rom[0] = 0x18
	rom[1] = 0xFE
	return rom
}

func TestPauseMatchesUnpausedRun(t *testing.T) {
	a := New()
	b := New()
	rom := jrROM()
	if err := a.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	if err := b.LoadROM(append([]byte(nil), rom...)); err != nil {
		t.Fatal(err)
	}
	var fr Frame
	for i := 0; i < 5; i++ {
		fr, _ = a.RunFrame()
		b.RunFrame()
	}
	pc, r, ft := a.cpu.PC, a.cpu.R, a.frameT
	ram := a.mem.RAM
	a.SetPaused(true)
	if a.Status() != "paused" {
		t.Fatal(a.Status())
	}
	for i := 0; i < 3; i++ {
		got, samples := a.RunFrame()
		if samples != nil {
			t.Fatal("paused audio")
		}
		if got.Pix != fr.Pix || a.cpu.PC != pc || a.cpu.R != r || a.frameT != ft || a.mem.RAM != ram {
			t.Fatal("paused machine moved")
		}
	}
	var host [8]byte
	host[2] = 1 // Q
	a.SetHostKeys(host)
	a.RunFrame()
	if a.keys.Read(0xFB) != 0x1E {
		t.Fatalf("keys while paused %05b", a.keys.Read(0xFB))
	}
	a.SetPaused(false)
	if a.Status() != "running" {
		t.Fatal(a.Status())
	}
	for i := 0; i < 4; i++ {
		a.RunFrame()
	}
	// The extra paused RunFrame above did not advance time, but SetPaused(false)
	// was followed by 4 frames. b has had 5 so far; run 4 more.
	for i := 0; i < 4; i++ {
		b.RunFrame()
	}
	if a.cpu.PC != b.cpu.PC || a.cpu.R != b.cpu.R || a.frameT != b.frameT || a.mem.RAM != b.mem.RAM {
		t.Fatalf("resume diverged PC %04X/%04X R %02X/%02X t %d/%d", a.cpu.PC, b.cpu.PC, a.cpu.R, b.cpu.R, a.frameT, b.frameT)
	}
}

func TestAutoloadSchedule(t *testing.T) {
	m := New()
	if err := m.LoadROM(jrROM()); err != nil {
		t.Fatal(err)
	}
	m.ArmTape([]Pulse{{Level: true, Dur: 100000}}, false, true)
	for i := 0; i < 3; i++ {
		m.RunFrame()
	}
	if m.framesRun != 3 {
		t.Fatalf("frames %d", m.framesRun)
	}
	check := func(at int, want byte, row byte) {
		t.Helper()
		m.framesRun = at
		m.RunFrame()
		if got := m.keys.Read(row); got != want {
			t.Fatalf("frame %d row %02X low %05b want %05b", at, row, got, want)
		}
	}
	check(150, 0x1F^0x08, 0xBF) // J is the LOAD keyword
	check(157, 0x1F^0x08, 0xBF)
	check(158, 0x1F, 0xBF)
	check(166, 0x1F^0x01, 0xDF) // P
	sym := m.keys.down[7]
	if sym&0x02 == 0 {
		t.Fatal("symbol shift")
	}
	check(198, 0x1F^0x01, 0xBF) // Enter
	check(206, 0x1F, 0xBF)

	off := New()
	if err := off.LoadROM(jrROM()); err != nil {
		t.Fatal(err)
	}
	off.ArmTape([]Pulse{{Level: true, Dur: 10}}, false, false)
	off.framesRun = 150
	off.RunFrame()
	if off.keys.Read(0xBF) != 0x1F {
		t.Fatal("autoload off still typed")
	}
}

func TestTapeCuedUntilLoad(t *testing.T) {
	m := New()
	m.ArmTape([]Pulse{{Level: true, Dur: 2168}, {Level: false, Dur: 2168}}, false, true)
	m.Advance(100000)
	if m.TapePlaying() || m.IORead(0xFE)&0x40 != 0 {
		t.Fatal("leader played before LOAD")
	}
	m.framesRun = 198
	m.installKeys()
	if !m.TapePlaying() || m.IORead(0xFE)&0x40 == 0 {
		t.Fatal("Enter should start the leader high")
	}
	m.Advance(2168)
	if m.IORead(0xFE)&0x40 != 0 {
		t.Fatal("cue consumed the first pulse")
	}

	manual := New()
	manual.ArmTape([]Pulse{{Level: true, Dur: 100}}, false, false)
	manual.Advance(69888)
	if manual.TapePlaying() || manual.IORead(0xFE)&0x40 != 0 {
		t.Fatal("manual tape rolled before Enter")
	}
	var host [8]byte
	host[6] = 0x01
	manual.SetHostKeys(host)
	manual.installKeys()
	if !manual.TapePlaying() || manual.IORead(0xFE)&0x40 == 0 {
		t.Fatal("host Enter should start the tape")
	}
}

func TestTurboDropsAudio(t *testing.T) {
	m := New()
	if err := m.LoadROM(jrROM()); err != nil {
		t.Fatal(err)
	}
	m.beep.Acc = 3000000
	m.SetTurbo(true)
	_, samples := m.RunFrame()
	if samples != nil {
		t.Fatal("turbo samples")
	}
	m.SetTurbo(false)
	if m.beep.Acc != 0 {
		t.Fatalf("acc %d", m.beep.Acc)
	}
	_, samples = m.RunFrame()
	if len(samples) == 0 {
		t.Fatal("audio resumes after turbo")
	}
}
