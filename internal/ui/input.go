package ui

import (
	"github.com/hajimehoshi/ebiten/v2"

	"go.spectrum/internal/machine"
)

// Input polls host keys into the 8×5 matrix. F1 is an edge, not a matrix bit.
// Ctrl and Cmd are never matrix bits, and while Cmd is held no key reaches
// the Spectrum, so host shortcuts do not type. A repeated poll while a key
// stays down does not add another press.
type Input struct {
	prevF1 bool
	// ArrowsJoystick sends the arrow keys and Tab to the Kempston port
	// instead of Caps Shift with 5 to 8.
	ArrowsJoystick bool
}

// Keys is one poll of the host keyboard.
type Keys struct {
	Rows        [8]byte
	Joystick    byte
	TogglePause bool
	Quit        bool
}

// Poll reads the current level of each host key.
func (in *Input) Poll(down func(string) bool) Keys {
	var out Keys
	f1 := down("F1")
	out.TogglePause = f1 && !in.prevF1
	in.prevF1 = f1
	out.Quit = down("Q") && (down("Control") || down("ControlLeft") || down("ControlRight"))
	if down("Meta") {
		return out
	}
	arrows := !in.ArrowsJoystick
	if in.ArrowsJoystick {
		for _, j := range []struct {
			key string
			bit byte
		}{{"ArrowRight", machine.JoyRight}, {"ArrowLeft", machine.JoyLeft}, {"ArrowDown", machine.JoyDown}, {"ArrowUp", machine.JoyUp}, {"Tab", machine.JoyFire}} {
			if down(j.key) {
				out.Joystick |= j.bit
			}
		}
	}
	var k machine.Keyboard
	if down("ShiftLeft") || down("Backspace") || (arrows && (down("ArrowLeft") || down("ArrowRight") || down("ArrowUp") || down("ArrowDown"))) {
		k.Press("CAPS")
	}
	if down("ShiftRight") || down("Alt") || down("AltLeft") || down("AltRight") {
		k.Press("SYMBOL")
	}
	if arrows && down("ArrowLeft") {
		k.Press("5")
	}
	if arrows && down("ArrowRight") {
		k.Press("8")
	}
	if arrows && down("ArrowUp") {
		k.Press("7")
	}
	if arrows && down("ArrowDown") {
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
	out.Rows = k.Rows()
	return out
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
	case "Tab":
		return ebiten.IsKeyPressed(ebiten.KeyTab)
	case "Meta":
		return ebiten.IsKeyPressed(ebiten.KeyMeta)
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

// GamepadJoystick reads every connected pad as one Kempston stick: the d-pad
// or left stick steers and any face button fires.
func GamepadJoystick() byte {
	const dead = 0.5
	var bits byte
	for _, id := range ebiten.AppendGamepadIDs(nil) {
		var x, y float64
		var left, right, up, down, fire bool
		if ebiten.IsStandardGamepadLayoutAvailable(id) {
			x = ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
			y = ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
			left = ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftLeft)
			right = ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftRight)
			up = ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftTop)
			down = ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftBottom)
			for _, b := range []ebiten.StandardGamepadButton{
				ebiten.StandardGamepadButtonRightBottom, ebiten.StandardGamepadButtonRightRight,
				ebiten.StandardGamepadButtonRightLeft, ebiten.StandardGamepadButtonRightTop,
			} {
				fire = fire || ebiten.IsStandardGamepadButtonPressed(id, b)
			}
		} else {
			x = ebiten.GamepadAxisValue(id, 0)
			y = ebiten.GamepadAxisValue(id, 1)
			fire = ebiten.IsGamepadButtonPressed(id, ebiten.GamepadButton0) || ebiten.IsGamepadButtonPressed(id, ebiten.GamepadButton1)
		}
		if left || x < -dead {
			bits |= machine.JoyLeft
		}
		if right || x > dead {
			bits |= machine.JoyRight
		}
		if up || y < -dead {
			bits |= machine.JoyUp
		}
		if down || y > dead {
			bits |= machine.JoyDown
		}
		if fire {
			bits |= machine.JoyFire
		}
	}
	return bits
}
