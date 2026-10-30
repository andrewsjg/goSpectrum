package ui

import "testing"

func held(keys ...string) func(string) bool {
	set := map[string]bool{}
	for _, k := range keys {
		set[k] = true
	}
	return func(name string) bool { return set[name] }
}

func TestHostKeys(t *testing.T) {
	var in Input
	rows, toggle, quit := in.Poll(held("ShiftLeft"))
	if toggle || quit || rows != [8]byte{0x01} {
		t.Fatalf("left shift rows %v toggle %v quit %v", rows, toggle, quit)
	}

	rows, _, _ = in.Poll(held("ShiftRight"))
	if rows != [8]byte{0, 0, 0, 0, 0, 0, 0, 0x02} {
		t.Fatalf("right shift %v", rows)
	}
	rows, _, _ = in.Poll(held("Alt"))
	if rows[7] != 0x02 {
		t.Fatalf("alt %v", rows)
	}

	rows, _, _ = in.Poll(held("ArrowUp"))
	if rows[0] != 0x01 || rows[4] != 0x08 {
		t.Fatalf("arrow up %v", rows)
	}
	rows, _, _ = in.Poll(held("Backspace"))
	if rows[0] != 0x01 || rows[4] != 0x01 {
		t.Fatalf("backspace %v", rows)
	}

	rows, toggle, quit = in.Poll(held("F1"))
	if !toggle || quit || rows != [8]byte{} {
		t.Fatalf("F1 edge rows %v toggle %v", rows, toggle)
	}
	rows, toggle, _ = in.Poll(held("F1"))
	if toggle || rows != [8]byte{} {
		t.Fatal("F1 repeat counted as another press")
	}
	_, toggle, _ = in.Poll(held())
	if toggle {
		t.Fatal("release toggled")
	}

	rows, _, quit = in.Poll(held("Control", "Q"))
	if !quit || rows[2] != 0x01 {
		t.Fatalf("ctrl+q quit %v rows %v", quit, rows)
	}
	var other byte
	for i, r := range rows {
		if i != 2 {
			other |= r
		}
	}
	if other != 0 {
		t.Fatal("ctrl set a matrix bit")
	}

	rows, _, _ = in.Poll(held("A", "S"))
	if rows[1] != 0x03 {
		t.Fatalf("two keys %v", rows)
	}
	again, _, _ := in.Poll(held("A", "S"))
	if again != rows {
		t.Fatal("repeat added a press")
	}
}
