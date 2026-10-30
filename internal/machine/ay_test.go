package machine

import "testing"

func TestAYPortsToneAndEnvelope(t *testing.T) {
	m := new128(t)
	m.IOWrite(0xFFFD, 7)
	m.IOWrite(0xBFFD, 0xFE)
	m.IOWrite(0xFFFD, 8)
	m.IOWrite(0xBFFD, 15)
	m.IOWrite(0xFFFD, 0)
	m.IOWrite(0xBFFD, 1)
	if m.IORead(0xFFFD) != 1 || m.IORead(0xBFFD) != 0xFF {
		t.Fatalf("select %02X data %02X", m.IORead(0xFFFD), m.IORead(0xBFFD))
	}
	m.IOWrite(0xFFFD, 14)
	m.IOWrite(0xBFFD, 0x12)
	if m.IORead(0xFFFD) != 0xFF {
		t.Fatalf("reg 14 %02X", m.IORead(0xFFFD))
	}

	// Period 1 toggles every 32 T-states. The first step turns the tone on.
	var levels []int16
	for i := 0; i < 32*4; i++ {
		m.ay.tick()
		if (i+1)%32 == 0 {
			levels = append(levels, m.ay.level)
		}
	}
	if len(levels) != 4 || levels[0] != ayLog[15] || levels[1] != 0 || levels[2] != ayLog[15] || levels[3] != 0 {
		t.Fatalf("tone %v", levels)
	}

	var env ayChip
	env.reset()
	env.reg[13] = 0
	if env.envAmp() != 15 {
		t.Fatal("decay starts high")
	}
	for i := 0; i < 16; i++ {
		env.stepEnv()
	}
	if !env.ehold || env.envAmp() != 0 {
		t.Fatal("shape 0 holds 0")
	}

	env.reset()
	env.reg[13] = 8
	for i := 0; i < 16; i++ {
		env.stepEnv()
	}
	if env.ehold || env.envAmp() != 15 {
		t.Fatalf("shape 8 repeats, hold %v amp %d", env.ehold, env.envAmp())
	}

	env.reset()
	env.reg[13] = 11
	for i := 0; i < 16; i++ {
		env.stepEnv()
	}
	if !env.ehold || env.envAmp() != 15 {
		t.Fatal("shape 11 holds 15")
	}

	env.reset()
	env.reg[13] = 13
	if env.envAmp() != 0 {
		t.Fatal("attack starts low")
	}
	for i := 0; i < 16; i++ {
		env.stepEnv()
	}
	if !env.ehold || env.envAmp() != 15 {
		t.Fatal("shape 13 holds 15")
	}
}
