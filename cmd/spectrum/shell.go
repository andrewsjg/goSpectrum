package main

import (
	"fmt"
	"io"
	"io/fs"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"go.spectrum/internal/config"
	"go.spectrum/internal/machine"
	"go.spectrum/internal/ui"
)

// shell is the one window. It shows the chooser or a running machine and
// moves between them. RunGame is called once, and one Audio serves every
// machine because Ebiten allows a single audio context per process.
type shell struct {
	pick   *pickerGame // nil until the chooser is first shown
	app    *app
	audio  *ui.Audio
	exe    string
	work   string
	cfgDir string // empty when there is none: no saves, recents, or chooser
	stderr io.Writer
	flags  cliFlags
	code   int
}

func (s *shell) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) || (ebiten.IsKeyPressed(ebiten.KeyMeta) && inpututil.IsKeyJustPressed(ebiten.KeyF)) {
		toggleFullscreen()
	}
	if name, data, ok := droppedFile(); ok {
		s.drop(name, data)
	}
	if s.app != nil {
		if err := s.app.Update(); err != nil {
			return err
		}
		switch s.app.want {
		case wantQuit:
			return ebiten.Termination
		case wantChooser:
			s.showChooser()
		case wantRestart:
			s.app.want = wantNone
			s.restart(s.app.launch)
		}
		return nil
	}
	if err := s.pick.Update(); err != nil {
		return err
	}
	p := s.pick.p
	if !p.done {
		return nil
	}
	res := p.result
	if res.Quit || !res.Go {
		return ebiten.Termination
	}
	l, err := resultToLaunch(res, s.cfgDir, s.flags)
	if err != nil {
		fmt.Fprintf(s.stderr, "spectrum: %v\n", err)
		p.fail(err.Error())
		return nil
	}
	sess, err := buildSession(l, s.exe, s.work)
	if err != nil {
		fmt.Fprintf(s.stderr, "spectrum: %v\n", err)
		if stayOnPicker(err) {
			p.fail(err.Error())
			return nil
		}
		s.code = 1
		return ebiten.Termination
	}
	s.reportDecode(sess)
	s.enter(sess, true)
	return nil
}

func (s *shell) Draw(screen *ebiten.Image) {
	if s.app != nil {
		s.app.Draw(screen)
		return
	}
	s.pick.Draw(screen)
}

func (s *shell) Layout(w, h int) (int, int) {
	if s.app != nil {
		return s.app.Layout(w, h)
	}
	return s.pick.Layout(w, h)
}

// enter starts sess. resize sets the window for the machine, which a
// restart or a drop onto a running game leaves as the user sized it.
func (s *shell) enter(sess *Session, resize bool) {
	if s.audio == nil {
		s.audio = ui.OpenAudio(sess.Sound)
	} else {
		s.audio.SetEnabled(sess.Sound)
	}
	if resize {
		prepareWindow(sess.Scale)
	}
	a := newApp(sess, s.audio, s.cfgDir, s.cfgDir != "")
	if s.app != nil {
		a.input.ArrowsJoystick = s.app.input.ArrowsJoystick
		a.quiet = s.app.quiet
	}
	s.app = a
	if s.cfgDir != "" && sess.Path != "" {
		model := "48"
		if sess.Mach.Is128() {
			model = "128"
		}
		if err := config.AddRecent(s.cfgDir, config.Recent{Model: model, Path: sess.Path}); err != nil {
			fmt.Fprintf(s.stderr, "spectrum: %v\n", err)
		}
	}
}

// restart rebuilds a launch over the running game. A failure keeps the game
// running and says why.
func (s *shell) restart(l launch) {
	sess, err := buildSession(l, s.exe, s.work)
	if err != nil {
		s.app.say(err.Error(), true)
		return
	}
	s.reportDecode(sess)
	s.enter(sess, false)
}

// reportDecode prints a program that did not decode. Execute does this for
// a direct start.
func (s *shell) reportDecode(sess *Session) {
	if sess.ErrorText != "" {
		fmt.Fprintf(s.stderr, "spectrum: %s\n", sess.ErrorText)
	}
}

// drop starts a file dropped on the window. Over a game it keeps that
// machine's options; over the chooser it uses the chosen model. A 128K
// snapshot may still switch a 48K choice to 128K.
func (s *shell) drop(name string, data []byte) {
	if s.app == nil {
		l := mergeLaunch(nil, s.flags)
		l.Model = s.pick.p.model
		l.File, l.Data, l.ModelExplicit = name, data, false
		sess, err := buildSession(l, s.exe, s.work)
		if err != nil {
			s.pick.p.fail(err.Error())
			return
		}
		s.reportDecode(sess)
		s.enter(sess, true)
		return
	}
	l := s.app.launch
	l.File, l.Data, l.ModelExplicit = name, data, false
	s.restart(l)
}

// showChooser leaves the game for the chooser, which is created the first
// time from a direct start.
func (s *shell) showChooser() {
	if s.pick == nil {
		p, err := newPicker(s.work, s.cfgDir)
		if err != nil {
			s.app.want = wantNone
			s.app.say(err.Error(), true)
			return
		}
		s.pick = newPickerGame(p, ui.NewFont(s.app.m.Charset()))
	} else {
		s.pick.p.reopen()
	}
	s.app = nil
	s.audio.SetEnabled(false)
	setChooserWindow()
}

// droppedFile returns the first file dropped on the window this frame.
func droppedFile() (string, []byte, bool) {
	fsys := ebiten.DroppedFiles()
	if fsys == nil {
		return "", nil, false
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return "", nil, false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return "", nil, false
		}
		return e.Name(), data, true
	}
	return "", nil, false
}

func toggleFullscreen() { ebiten.SetFullscreen(!ebiten.IsFullscreen()) }

func prepareWindow(scale int) {
	w, h := ui.WindowSize(scale)
	ebiten.SetWindowSize(w, h)
	ebiten.SetWindowSizeLimits(machine.VisibleW, machine.VisibleH, -1, -1)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowClosingHandled(true)
}

func setChooserWindow() {
	ebiten.SetWindowSize(pickerW*2, pickerH*2)
	ebiten.SetWindowSizeLimits(pickerW, pickerH, -1, -1)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowClosingHandled(true)
	ebiten.SetWindowTitle("goSpectrum")
}

// runChooser opens the window on the chooser.
func runChooser(exe, work, cfgDir string, stderr io.Writer, flags cliFlags) int {
	p, err := newPicker(work, cfgDir)
	if err != nil {
		fmt.Fprintf(stderr, "spectrum: %v\n", err)
		return 1
	}
	sh := &shell{
		pick:   newPickerGame(p, chooserFont(exe, work)),
		exe:    exe,
		work:   work,
		cfgDir: cfgDir,
		stderr: stderr,
		flags:  flags,
	}
	setChooserWindow()
	return sh.run()
}

// runDirect opens the window on a session built from the command line.
func runDirect(sess *Session, exe, work, cfgDir string, stderr io.Writer, flags cliFlags) int {
	sh := &shell{exe: exe, work: work, cfgDir: cfgDir, stderr: stderr, flags: flags}
	sh.enter(sess, true)
	return sh.run()
}

func (s *shell) run() int {
	err := ebiten.RunGame(s)
	if s.audio != nil {
		s.audio.Close()
	}
	if s.code != 0 {
		return s.code
	}
	if err != nil {
		fmt.Fprintf(s.stderr, "spectrum: %v\n", err)
		return 1
	}
	return 0
}
