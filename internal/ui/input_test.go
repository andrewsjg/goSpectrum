package ui

import (
	"testing"

	"go.spectrum/internal/machine"
)

func held(keys ...string) func(string) bool {
	set := map[string]bool{}
	for _, k := range keys {
		set[k] = true
	}
	return func(name string) bool { return set[name] }
}

func TestHostKeys(t *testing.T) {
	var in Input
	k := in.Poll(held("ShiftLeft"))
	if k.TogglePause || k.Quit || k.Rows != [8]byte{0x01} {
		t.Fatalf("left shift %+v", k)
	}

	k = in.Poll(held("ShiftRight"))
	if k.Rows != [8]byte{0, 0, 0, 0, 0, 0, 0, 0x02} {
		t.Fatalf("right shift %v", k.Rows)
	}
	k = in.Poll(held("Alt"))
	if k.Rows[7] != 0x02 {
		t.Fatalf("alt %v", k.Rows)
	}

	k = in.Poll(held("ArrowUp"))
	if k.Rows[0] != 0x01 || k.Rows[4] != 0x08 || k.Joystick != 0 {
		t.Fatalf("arrow up %+v", k)
	}
	k = in.Poll(held("Backspace"))
	if k.Rows[0] != 0x01 || k.Rows[4] != 0x01 {
		t.Fatalf("backspace %v", k.Rows)
	}

	k = in.Poll(held("F1"))
	if !k.TogglePause || k.Quit || k.Rows != [8]byte{} {
		t.Fatalf("F1 edge %+v", k)
	}
	k = in.Poll(held("F1"))
	if k.TogglePause || k.Rows != [8]byte{} {
		t.Fatal("F1 repeat counted as another press")
	}
	if in.Poll(held()).TogglePause {
		t.Fatal("release toggled")
	}

	k = in.Poll(held("Control", "Q"))
	if !k.Quit || k.Rows[2] != 0x01 {
		t.Fatalf("ctrl+q %+v", k)
	}
	var other byte
	for i, r := range k.Rows {
		if i != 2 {
			other |= r
		}
	}
	if other != 0 {
		t.Fatal("ctrl set a matrix bit")
	}

	k = in.Poll(held("A", "S"))
	if k.Rows[1] != 0x03 {
		t.Fatalf("two keys %v", k.Rows)
	}
	if again := in.Poll(held("A", "S")); again.Rows != k.Rows {
		t.Fatal("repeat added a press")
	}

	if k = in.Poll(held("Meta", "F")); k.Rows != [8]byte{} {
		t.Fatalf("cmd shortcut reached the matrix %v", k.Rows)
	}
}

func TestArrowsJoystick(t *testing.T) {
	in := Input{ArrowsJoystick: true}
	k := in.Poll(held("ArrowLeft", "ArrowUp", "Tab"))
	if k.Joystick != machine.JoyLeft|machine.JoyUp|machine.JoyFire {
		t.Fatalf("joystick %05b", k.Joystick)
	}
	if k.Rows != [8]byte{} {
		t.Fatalf("arrows also pressed keys %v", k.Rows)
	}
	if k = in.Poll(held("Backspace")); k.Rows[0] != 0x01 || k.Rows[4] != 0x01 {
		t.Fatal("backspace stopped deleting in joystick mode")
	}
}
