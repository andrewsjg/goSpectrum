package main

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"go.spectrum/internal/config"
	"go.spectrum/internal/formats"
	"go.spectrum/internal/machine"
	"go.spectrum/internal/timing"
	"go.spectrum/internal/ui"
)

// request is what a running game asks the shell to do after an Update.
type request int

const (
	wantNone request = iota
	wantRestart
	wantChooser
	wantQuit
)

// Toast lifetimes in updates, at 60 per second.
const (
	toastTicks    = 150
	toastBadTicks = 300
)

// app runs one machine. F1 toggles pause, Esc opens the menu, F5 and F9
// save and load the program's state. Ctrl+Q and the close button exit 0.
// Overlays are drawn into the frame image, so they scale with the Spectrum.
type app struct {
	m        *machine.Machine
	audio    *ui.Audio
	input    ui.Input
	clock    timing.Clock
	font     *ui.Font
	name     string
	path     string
	launch   launch
	cfgDir   string // empty: no save states
	chooser  bool   // the menu offers Choose another game
	realtime bool
	screen   *ebiten.Image
	frame    machine.Frame
	have     bool
	last     time.Time
	winW     int
	winH     int

	menu     *gameMenu // nil while closed
	quiet    bool      // overlay off: only the menu and error toasts draw
	holdKeys bool      // keys from the menu stay off the matrix until released
	mouseX   int
	mouseY   int
	toast    string
	toastBad bool
	toastT   int
	want     request
}

// newApp starts audio for the session and runs its first frame.
func newApp(s *Session, aud *ui.Audio, cfgDir string, chooser bool) *app {
	a := &app{
		m:        s.Mach,
		audio:    aud,
		font:     ui.NewFont(s.Mach.Charset()),
		name:     s.Name,
		path:     s.Path,
		launch:   s.Launch,
		cfgDir:   cfgDir,
		chooser:  chooser,
		realtime: s.RealtimeTape,
		screen:   ebiten.NewImage(machine.VisibleW, machine.VisibleH),
	}
	turbo := s.Mach.TapePlaying() && !s.RealtimeTape
	s.Mach.SetTurbo(turbo)
	a.clock.SetTurbo(turbo)
	fr, samples := s.Mach.RunFrame()
	if !turbo {
		aud.Push(samples)
	}
	a.frame = fr
	a.have = true
	if s.ErrorText != "" {
		a.say(s.ErrorText, true)
	} else {
		a.say("Esc menu   F5 save   F9 load", false)
		a.toastT = toastBadTicks // outlasts the boot screen
	}
	a.setTitle()
	return a
}

func (a *app) setTitle() {
	ebiten.SetWindowTitle(ui.Title(a.name, a.m.Status(), a.m.ErrorText(), a.audio.Notice()))
}

func (a *app) say(msg string, bad bool) {
	a.toast, a.toastBad = msg, bad
	a.toastT = toastTicks
	if bad {
		a.toastT = toastBadTicks
	}
}

func (a *app) Update() error {
	if ebiten.IsWindowBeingClosed() {
		return ebiten.Termination
	}
	if a.toastT > 0 {
		a.toastT--
	}
	defer a.setTitle()
	if a.menu != nil {
		a.updateMenu()
		a.m.SetHostKeys([8]byte{})
		a.m.SetJoystick(0)
		a.m.RunFrame()
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.openMenu()
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
		a.saveState()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF9) {
		a.loadState()
	}
	keys := a.input.Poll(ui.EbitenDown)
	if keys.Quit {
		return ebiten.Termination
	}
	joy := keys.Joystick | ui.GamepadJoystick()
	if a.holdKeys {
		if keys.Rows != ([8]byte{}) || joy != 0 {
			keys.Rows, joy = [8]byte{}, 0
		} else {
			a.holdKeys = false
		}
	}
	a.m.SetHostKeys(keys.Rows)
	a.m.SetJoystick(joy)
	if keys.TogglePause {
		a.m.TogglePause()
	}
	turbo := a.m.TapePlaying() && !a.realtime
	if turbo != a.clock.Turbo() {
		a.clock.SetTurbo(turbo)
	}
	a.m.SetTurbo(turbo)
	if a.m.Paused() {
		a.m.RunFrame()
		return nil
	}
	if turbo {
		deadline := time.Now().Add(12 * time.Millisecond)
		for n := 0; n < 400 && time.Now().Before(deadline); n++ {
			a.frame, _ = a.m.RunFrame()
			a.have = true
		}
		return nil
	}
	now := time.Now()
	var dt int64
	if !a.last.IsZero() {
		dt = now.Sub(a.last).Nanoseconds()
	}
	a.last = now
	n := a.clock.Tick(dt, false)
	for i := 0; i < n; i++ {
		var samples []int16
		a.frame, samples = a.m.RunFrame()
		a.have = true
		a.audio.Push(samples)
	}
	return nil
}

func (a *app) Draw(screen *ebiten.Image) {
	if !a.have {
		screen.Fill(color.Black)
		return
	}
	a.screen.WritePixels(ui.FrameRGBA(a.frame))
	a.drawOverlay(a.screen)
	ui.DrawFrame(screen, a.screen)
}

func (a *app) Layout(w, h int) (int, int) {
	a.winW, a.winH = w, h
	return w, h
}

// Save states

// statePath is the save-state file for this program. A program started
// from the save folder saves over itself. Otherwise the original extension
// is kept as a tag, so a .tap and a .sna of one game save apart.
func statePath(cfgDir, name, path string) string {
	dir := config.SavesDir(cfgDir)
	if path != "" && filepath.Dir(path) == dir {
		return path
	}
	if name == "" {
		return filepath.Join(dir, "BASIC.z80")
	}
	ext := filepath.Ext(name)
	tag := strings.ToLower(strings.TrimPrefix(ext, "."))
	return filepath.Join(dir, strings.TrimSuffix(name, ext)+" ["+tag+"].z80")
}

func (a *app) saveState() {
	if a.cfgDir == "" {
		a.say("No config folder to save into", true)
		return
	}
	path := statePath(a.cfgDir, a.name, a.path)
	data := formats.WriteZ80(fromSnap(a.m.Snapshot()))
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err == nil {
		err = os.WriteFile(path, data, 0o644)
	}
	if err != nil {
		a.say(err.Error(), true)
		return
	}
	a.say("State saved", false)
}

func (a *app) loadState() {
	if a.cfgDir == "" {
		a.say("No config folder to load from", true)
		return
	}
	data, err := os.ReadFile(statePath(a.cfgDir, a.name, a.path))
	if os.IsNotExist(err) {
		a.say("No saved state yet - F5 saves one", true)
		return
	}
	if err != nil {
		a.say(err.Error(), true)
		return
	}
	snap, err := formats.ReadZ80(data)
	if err != nil {
		a.say(err.Error(), true)
		return
	}
	if snap.Machine128 && !a.m.Is128() {
		a.say("That state needs a 128K", true)
		return
	}
	a.m.ApplySnapshot(toSnap(snap))
	a.say("State loaded", false)
}

// Menu

type menuItem int

const (
	itemResume menuItem = iota
	itemSave
	itemLoad
	itemRestart
	itemSound
	itemArrows
	itemOverlay
	itemFullscreen
	itemChooser
	itemQuit
)

// gameMenu is the Esc overlay. The machine is paused while it is open and
// gets back the pause state it had.
type gameMenu struct {
	items     []menuItem
	cursor    int
	wasPaused bool
}

func newGameMenu(chooser, wasPaused bool) *gameMenu {
	items := []menuItem{itemResume, itemSave, itemLoad, itemRestart, itemSound, itemArrows, itemOverlay, itemFullscreen}
	if chooser {
		items = append(items, itemChooser)
	}
	items = append(items, itemQuit)
	return &gameMenu{items: items, wasPaused: wasPaused}
}

func (g *gameMenu) move(d int) {
	n := len(g.items)
	g.cursor = (g.cursor + d + n) % n
}

// Menu geometry in frame pixels.
const (
	menuW    = 232
	menuHead = 36
	menuRowH = 18
	menuFoot = 24
)

func menuBox(n int) box {
	h := menuHead + n*menuRowH + menuFoot
	return box{(machine.VisibleW - menuW) / 2, (machine.VisibleH - h) / 2, menuW, h}
}

func menuRowBox(b box, i int) box {
	return box{b.x + 6, b.y + menuHead + i*menuRowH, b.w - 12, menuRowH}
}

func (a *app) openMenu() {
	a.menu = newGameMenu(a.chooser, a.m.Paused())
	a.m.SetPaused(true)
	a.mouseX, a.mouseY = ebiten.CursorPosition()
}

func (a *app) closeMenu() {
	a.m.SetPaused(a.menu.wasPaused)
	a.menu = nil
	a.holdKeys = true
}

func (a *app) updateMenu() {
	g := a.menu
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		a.closeMenu()
		return
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowUp):
		g.move(-1)
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowDown):
		g.move(1)
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter), inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter), inpututil.IsKeyJustPressed(ebiten.KeySpace):
		a.activate(g.items[g.cursor])
		return
	}
	// The mouse moves the cursor only when it moves, so it does not fight
	// the arrow keys.
	x, y := ebiten.CursorPosition()
	moved := x != a.mouseX || y != a.mouseY
	a.mouseX, a.mouseY = x, y
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	if !moved && !click {
		return
	}
	s, ox, oy := ui.FrameOrigin(a.winW, a.winH)
	if x < ox || y < oy {
		return
	}
	lx, ly := (x-ox)/s, (y-oy)/s
	b := menuBox(len(g.items))
	for i := range g.items {
		if menuRowBox(b, i).contains(lx, ly) {
			g.cursor = i
			if click {
				a.activate(g.items[i])
			}
			return
		}
	}
}

func (a *app) activate(it menuItem) {
	switch it {
	case itemResume:
		a.closeMenu()
	case itemSave:
		a.closeMenu()
		a.saveState()
	case itemLoad:
		a.closeMenu()
		a.loadState()
	case itemRestart:
		a.closeMenu()
		a.want = wantRestart
	case itemSound:
		a.audio.SetEnabled(!a.audio.Enabled())
		a.launch.Sound = a.audio.Enabled()
		switch {
		case a.audio.Enabled():
			a.say("Sound on", false)
		case a.audio.Notice() == "sound off":
			a.say("Sound off", false)
		default:
			a.say("Sound unavailable", true)
		}
	case itemArrows:
		a.input.ArrowsJoystick = !a.input.ArrowsJoystick
		if a.input.ArrowsJoystick {
			a.say("Arrows are a Kempston stick, Tab fires", false)
		} else {
			a.say("Arrows are cursor keys", false)
		}
	case itemOverlay:
		a.quiet = !a.quiet
	case itemFullscreen:
		toggleFullscreen()
	case itemChooser:
		a.want = wantChooser
	case itemQuit:
		a.want = wantQuit
	}
}

func (a *app) itemText(it menuItem) (label, value string) {
	switch it {
	case itemResume:
		return "Resume", "Esc"
	case itemSave:
		return "Save state", "F5"
	case itemLoad:
		return "Load state", "F9"
	case itemRestart:
		return "Restart", ""
	case itemSound:
		if a.audio.Enabled() {
			return "Sound", "On"
		}
		return "Sound", "Off"
	case itemArrows:
		if a.input.ArrowsJoystick {
			return "Arrow keys", "Kempston"
		}
		return "Arrow keys", "Cursor"
	case itemOverlay:
		if a.quiet {
			return "Overlay", "Off"
		}
		return "Overlay", "On"
	case itemFullscreen:
		return "Fullscreen", "F11"
	case itemChooser:
		return "Choose another game", ""
	default:
		return "Quit", "Ctrl+Q"
	}
}

// Overlay

// drawOverlay draws the loading bar, pause banner, toasts, and menu. With
// the overlay off only the menu and error toasts draw, so failures still show.
func (a *app) drawOverlay(dst *ebiten.Image) {
	const w, h = machine.VisibleW, machine.VisibleH
	if a.m.TapePlaying() && !a.quiet {
		b := box{(w - 208) / 2, h - 30, 208, 18}
		fillRect(dst, b, colBar)
		strokeRect(dst, b, colBorder)
		a.font.Text(dst, "LOADING", b.x+6, b.y+5, 1, colAccent)
		bar := box{b.x + 70, b.y + 6, b.w - 78, 6}
		fillRect(dst, bar, colBG)
		fillRect(dst, box{bar.x, bar.y, int(float64(bar.w) * a.m.TapeProgress()), bar.h}, colAccent)
	}
	if a.menu != nil {
		a.drawMenu(dst)
	} else if a.m.Paused() && !a.quiet {
		b := box{(w - 176) / 2, (h - 44) / 2, 176, 44}
		fillRect(dst, b, colBar)
		strokeRect(dst, b, colAccent)
		centerText(a.font, dst, "PAUSED", b, b.y+8, 2, colText)
		centerText(a.font, dst, "F1 resume  Esc menu", b, b.y+30, 1, colDim)
	}
	if a.toastT > 0 && (a.toastBad || !a.quiet) {
		msg := fitText(a.toast, w/ui.GlyphW-4)
		tw := len([]rune(msg))*ui.GlyphW + 16
		b := box{(w - tw) / 2, 12, tw, 18}
		fg, edge := colText, colAccent
		if a.toastBad {
			fg, edge = colWarn, colWarn
		}
		fillRect(dst, b, colBar)
		strokeRect(dst, b, edge)
		a.font.Text(dst, msg, b.x+8, b.y+5, 1, fg)
	}
}

func (a *app) drawMenu(dst *ebiten.Image) {
	g := a.menu
	fillRect(dst, box{0, 0, machine.VisibleW, machine.VisibleH}, colShade)
	b := menuBox(len(g.items))
	fillRect(dst, b, colBar)
	strokeRect(dst, b, colAccent)
	a.font.Text(dst, "goSpectrum", b.x+10, b.y+8, 1, colText)
	for y := 0; y < 10; y++ {
		for i, c := range stripes {
			fillRect(dst, box{b.x + b.w - 40 + i*5 + (9 - y), b.y + 7 + y, 5, 1}, c)
		}
	}
	title := "BASIC"
	if a.name != "" {
		title, _ = prettyName(a.name)
	}
	a.font.Text(dst, fitText(title, (b.w-20)/ui.GlyphW), b.x+10, b.y+20, 1, colDim)
	for i, it := range g.items {
		r := menuRowBox(b, i)
		if i == g.cursor {
			fillRect(dst, r, colCursor)
			fillRect(dst, box{r.x, r.y, 2, r.h}, colAccent)
		}
		label, value := a.itemText(it)
		a.font.Text(dst, label, r.x+8, r.y+5, 1, colText)
		if value != "" {
			a.font.Text(dst, value, r.x+r.w-6-len(value)*ui.GlyphW, r.y+5, 1, colDim)
		}
	}
	foot := box{b.x, b.y + b.h - menuFoot, b.w, menuFoot}
	centerText(a.font, dst, "ENTER select  ESC close", foot, foot.y+9, 1, colDim)
}
