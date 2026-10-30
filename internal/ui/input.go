package ui

import (
	"github.com/hajimehoshi/ebiten/v2"

	"go.spectrum/internal/machine"
)

// Input polls host keys into the 8×5 matrix. F1 is an edge, not a matrix bit.
// Ctrl is never a matrix bit. A repeated poll while a key stays down does not
// add another press.
type Input struct {
	prevF1 bool
}

// Poll reads the current level of each host key.
func (in *Input) Poll(down func(string) bool) (rows [8]byte, togglePause, quit bool) {
	var k machine.Keyboard
	if down("ShiftLeft") || down("Backspace") || down("ArrowLeft") || down("ArrowRight") || down("ArrowUp") || down("ArrowDown") {
		k.Press("CAPS")
	}
	if down("ShiftRight") || down("Alt") || down("AltLeft") || down("AltRight") {
		k.Press("SYMBOL")
	}
	if down("ArrowLeft") {
		k.Press("5")
	}
	if down("ArrowRight") {
		k.Press("8")
	}
	if down("ArrowUp") {
		k.Press("7")
	}
	if down("ArrowDown") {
		k.Press("6")
	}
	if down("Backspace") {
		k.Press("0")
	}
	if down("Enter") {
		k.Press("ENTER")
	}
	if down("Space") {
		k.Press("SPACE")
	}
	for c := byte('A'); c <= 'Z'; c++ {
		if down(string(rune(c))) {
			k.Press(string(rune(c)))
		}
	}
	for c := byte('0'); c <= '9'; c++ {
		if down(string(rune(c))) {
			k.Press(string(rune(c)))
		}
	}
	f1 := down("F1")
	togglePause = f1 && !in.prevF1
	in.prevF1 = f1
	quit = down("Q") && (down("Control") || down("ControlLeft") || down("ControlRight"))
	return k.Rows(), togglePause, quit
}

// EbitenDown is the host key source for Poll. Ctrl and Alt match either side.
func EbitenDown(name string) bool {
	switch name {
	case "Alt", "AltLeft", "AltRight":
		return ebiten.IsKeyPressed(ebiten.KeyAlt) || ebiten.IsKeyPressed(ebiten.KeyAltLeft) || ebiten.IsKeyPressed(ebiten.KeyAltRight)
	case "Control", "ControlLeft", "ControlRight":
		return ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
	case "ShiftLeft":
		return ebiten.IsKeyPressed(ebiten.KeyShiftLeft)
	case "ShiftRight":
		return ebiten.IsKeyPressed(ebiten.KeyShiftRight)
	case "ArrowLeft":
		return ebiten.IsKeyPressed(ebiten.KeyArrowLeft)
	case "ArrowRight":
		return ebiten.IsKeyPressed(ebiten.KeyArrowRight)
	case "ArrowUp":
		return ebiten.IsKeyPressed(ebiten.KeyArrowUp)
	case "ArrowDown":
		return ebiten.IsKeyPressed(ebiten.KeyArrowDown)
	case "Backspace":
		return ebiten.IsKeyPressed(ebiten.KeyBackspace)
	case "Enter":
		return ebiten.IsKeyPressed(ebiten.KeyEnter)
	case "Space":
		return ebiten.IsKeyPressed(ebiten.KeySpace)
	case "F1":
		return ebiten.IsKeyPressed(ebiten.KeyF1)
	}
	if len(name) == 1 && name[0] >= 'A' && name[0] <= 'Z' {
		return ebiten.IsKeyPressed(ebiten.KeyA + ebiten.Key(name[0]-'A'))
	}
	if len(name) == 1 && name[0] >= '0' && name[0] <= '9' {
		return ebiten.IsKeyPressed(ebiten.KeyDigit0 + ebiten.Key(name[0]-'0'))
	}
	return false
}
