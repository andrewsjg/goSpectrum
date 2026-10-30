package machine

import "testing"

func TestBeeperAndULAPort(t *testing.T) {
	var border byte
	var mic, speaker bool
	if !WriteULA(&border, &mic, &speaker, 0xFE, 0x1F) {
		t.Fatal("bit 0 clear is the ULA")
	}
	if border != 7 || !mic || !speaker {
		t.Fatalf("latches border %d mic %v speaker %v", border, mic, speaker)
	}
	if WriteULA(&border, &mic, &speaker, 0xFF, 0) {
		t.Fatal("bit 0 set must not select the ULA")
	}
	if border != 7 || !speaker {
		t.Fatal("non-ULA write changed the speaker")
	}

	var b Beeper
	if len(b.Tick(72)) != 0 {
		t.Fatal("72 ticks are short of a sample")
	}
	got := b.Tick(1)
	if len(got) != 1 || got[0] != -AudioAmp {
		t.Fatalf("first sample %v", got)
	}
	b.Speaker = true
	b.Acc = 0
	got = b.Tick(73)
	if len(got) != 1 || got[0] != AudioAmp {
		t.Fatalf("high sample %v", got)
	}
}
