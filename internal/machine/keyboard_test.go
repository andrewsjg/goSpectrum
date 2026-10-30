package machine

import "testing"

func TestKeyboardMatrix(t *testing.T) {
	keys := []struct {
		name string
		sel  byte
		bit  uint
	}{
		{"CAPS", 0xFE, 0}, {"Z", 0xFE, 1}, {"X", 0xFE, 2}, {"C", 0xFE, 3}, {"V", 0xFE, 4},
		{"A", 0xFD, 0}, {"S", 0xFD, 1}, {"D", 0xFD, 2}, {"F", 0xFD, 3}, {"G", 0xFD, 4},
		{"Q", 0xFB, 0}, {"W", 0xFB, 1}, {"E", 0xFB, 2}, {"R", 0xFB, 3}, {"T", 0xFB, 4},
		{"1", 0xF7, 0}, {"2", 0xF7, 1}, {"3", 0xF7, 2}, {"4", 0xF7, 3}, {"5", 0xF7, 4},
		{"0", 0xEF, 0}, {"9", 0xEF, 1}, {"8", 0xEF, 2}, {"7", 0xEF, 3}, {"6", 0xEF, 4},
		{"P", 0xDF, 0}, {"O", 0xDF, 1}, {"I", 0xDF, 2}, {"U", 0xDF, 3}, {"Y", 0xDF, 4},
		{"ENTER", 0xBF, 0}, {"L", 0xBF, 1}, {"K", 0xBF, 2}, {"J", 0xBF, 3}, {"H", 0xBF, 4},
		{"SPACE", 0x7F, 0}, {"SYMBOL", 0x7F, 1}, {"M", 0x7F, 2}, {"N", 0x7F, 3}, {"B", 0x7F, 4},
	}
	for _, k := range keys {
		var kb Keyboard
		kb.Press(k.name)
		got := kb.Read(k.sel)
		want := byte(0x1F) ^ (1 << k.bit)
		if got != want {
			t.Fatalf("%s: low bits %05b, want %05b", k.name, got, want)
		}
	}
}

func TestKeyboardPortAndGhost(t *testing.T) {
	m := New()
	m.keys.Press("Q")
	if got := m.IORead(0xFBFE); got != 0xBE {
		t.Fatalf("Q low, tape low: %02X", got)
	}
	m.tape.Arm([]Pulse{{Level: true, Dur: 100}}, false)
	if got := m.IORead(0xFBFE); got != 0xFE {
		t.Fatalf("Q low, tape high: %02X", got)
	}
	var kb Keyboard
	kb.Press("Z")
	kb.Press("S")
	kb.Press("A")
	if got := kb.Read(0xFE); got != 0x1C {
		t.Fatalf("ghost %05b", got)
	}
}
