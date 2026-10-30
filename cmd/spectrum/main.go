// Command spectrum runs a ZX Spectrum 48K or 128K.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.spectrum/internal/config"
	"go.spectrum/internal/formats"
	"go.spectrum/internal/machine"
)

const version = "0.1.0"

const usageText = `goSpectrum runs a ZX Spectrum 48K or 128K.

Usage:
  spectrum [options] [file]

With no file, --model, --rom, or --config, a chooser opens.

Options:
  --model 48|128     machine. Direct start defaults to 48
  --rom <path>       16384-byte 48K ROM, or 32768-byte 128K ROM
  --config <name>    saved chooser config, or a path to one
  --scale <N>        integer window scale, default 2
  --sound <bool>     beeper and AY audio, default true
  --realtime-tape    play the tape at wall-clock speed
  --no-autoload      arm a tape without typing LOAD ""
  -h, --help         show this help
  --version          print the version
`

// Session is one running program: a machine plus the options that built it.
// Launch rebuilds it for Restart. Path is the absolute program file, empty
// for BASIC or a dropped file.
type Session struct {
	Launch       launch
	Path         string
	Mach         *machine.Machine
	Scale        int
	Sound        bool
	RealtimeTape bool
	NoAutoload   bool
	Name         string
	Notice       string
	ErrorText    string
}

// Option configures Execute. Start, when set, replaces the window.
// Tests pass Start so RunGame is never called.
// ConfigDir, when set, is where named configs are stored. Empty uses the
// user config directory.
// A start with no file, --model, --rom, or --config opens the chooser.
// If Start is also set, Execute returns 0 without calling Start and
// without opening a window.
type Option struct {
	Stdout    io.Writer
	Stderr    io.Writer
	Start     func(*Session) int
	ExeDir    string
	WorkDir   string
	ConfigDir string
}

func main() {
	exe, err := os.Executable()
	exeDir := "."
	if err == nil {
		exeDir = filepath.Dir(exe)
	}
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	os.Exit(Execute(os.Args[1:], Option{
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		ExeDir:  exeDir,
		WorkDir: wd,
	}))
}

// Execute parses args and either starts the window or returns before it.
func Execute(args []string, opt Option) int {
	stdout, stderr := opt.Stdout, opt.Stderr
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	work := opt.WorkDir
	if work == "" {
		work = "."
	}
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprint(stdout, usageText)
			return 0
		}
		if a == "--version" {
			fmt.Fprintf(stdout, "goSpectrum %s\n", version)
			return 0
		}
	}

	fs := flag.NewFlagSet("spectrum", flag.ContinueOnError)
	fs.SetOutput(stderr)
	modelFlag := fs.String("model", "", "48 or 128")
	romFlag := fs.String("rom", "", "ROM image")
	configFlag := fs.String("config", "", "saved config")
	scale := fs.Int("scale", 2, "window scale")
	sound := fs.Bool("sound", true, "audio")
	realtime := fs.Bool("realtime-tape", false, "real-time tape")
	noAuto := fs.Bool("no-autoload", false, "do not type LOAD")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	flags := cliFlags{
		model:    *modelFlag,
		modelSet: set["model"],
		rom:      *romFlag,
		romSet:   set["rom"],
		scale:    *scale,
		scaleSet: set["scale"],
		sound:    *sound,
		soundSet: set["sound"],
		realtime: *realtime,
		rtSet:    set["realtime-tape"],
		noAuto:   *noAuto,
		naSet:    set["no-autoload"],
	}
	if flags.modelSet && flags.model != "48" && flags.model != "128" {
		fmt.Fprintf(stderr, "spectrum: --model must be 48 or 128\n")
		return 1
	}
	if flags.scaleSet && flags.scale < 1 {
		fmt.Fprintf(stderr, "spectrum: --scale must be >= 1\n")
		return 1
	}
	files := fs.Args()
	if len(files) > 1 {
		fmt.Fprintf(stderr, "spectrum: too many files\n")
		return 1
	}
	if len(files) == 1 {
		flags.file = files[0]
	}

	chooser := flags.file == "" && *configFlag == "" && !flags.modelSet && !flags.romSet
	// The chooser and --config need the config directory. A direct start
	// runs without one and only loses saves, recents, and the chooser.
	cfgDir, cfgErr := config.Dir(opt.ConfigDir)
	if chooser {
		if opt.Start != nil {
			return 0
		}
		if cfgErr != nil {
			fmt.Fprintf(stderr, "spectrum: %v\n", cfgErr)
			return 1
		}
		return runChooser(opt.ExeDir, work, cfgDir, stderr, flags)
	}

	var cfg *config.File
	if *configFlag != "" {
		if cfgErr != nil {
			fmt.Fprintf(stderr, "spectrum: %v\n", cfgErr)
			return 1
		}
		path, err := config.Resolve(cfgDir, *configFlag)
		if err != nil {
			fmt.Fprintf(stderr, "spectrum: %v\n", err)
			return 1
		}
		loaded, err := config.Load(path)
		if err != nil {
			fmt.Fprintf(stderr, "spectrum: %v\n", err)
			return 1
		}
		cfg = &loaded
	}
	sess, err := buildSession(mergeLaunch(cfg, flags), opt.ExeDir, work)
	if err != nil {
		fmt.Fprintf(stderr, "spectrum: %v\n", err)
		return 1
	}
	if sess.ErrorText != "" {
		fmt.Fprintf(stderr, "spectrum: %s\n", sess.ErrorText)
	}
	if opt.Start != nil {
		return opt.Start(sess)
	}
	if cfgErr != nil {
		cfgDir = ""
	}
	return runDirect(sess, opt.ExeDir, work, cfgDir, stderr, flags)
}

// cliFlags records which options were present on the command line.
// An absent flag does not override a saved config.
type cliFlags struct {
	model    string
	modelSet bool
	rom      string
	romSet   bool
	scale    int
	scaleSet bool
	sound    bool
	soundSet bool
	realtime bool
	rtSet    bool
	noAuto   bool
	naSet    bool
	file     string
}

// launch is the machine a direct start or a chooser choice asked for.
// ModelExplicit stops a 128K snapshot from changing a chosen 48K machine.
// Data, when set, is the program itself and File is only its name; a file
// dropped on the window has no path.
type launch struct {
	Model         string
	ModelExplicit bool
	ROM           string
	Scale         int
	Sound         bool
	Realtime      bool
	NoAutoload    bool
	File          string
	Data          []byte
}

func mergeLaunch(cfg *config.File, f cliFlags) launch {
	l := launch{Model: "48", Scale: 2, Sound: true}
	if cfg != nil {
		l.Model = cfg.Model
		l.ModelExplicit = true
		l.ROM = cfg.ROM
		l.Scale = cfg.ScaleOr(2)
		l.Sound = cfg.SoundOn()
		l.Realtime = cfg.RealtimeTape
		l.NoAutoload = cfg.NoAutoload
		l.File = cfg.Tape
	}
	if f.modelSet {
		l.Model = f.model
		l.ModelExplicit = true
	}
	if l.Model == "" {
		l.Model = "48"
	}
	if f.romSet {
		l.ROM = f.rom
	}
	if f.scaleSet {
		l.Scale = f.scale
	}
	if f.soundSet {
		l.Sound = f.sound
	}
	if f.rtSet {
		l.Realtime = f.realtime
	}
	if f.naSet {
		l.NoAutoload = f.noAuto
	}
	if f.file != "" {
		l.File = f.file
	}
	if l.Scale < 1 {
		l.Scale = 2
	}
	return l
}

func resultToLaunch(res pickerResult, cfgDir string, f cliFlags) (launch, error) {
	if res.ConfigName != "" {
		path, err := config.Resolve(cfgDir, res.ConfigName)
		if err != nil {
			return launch{}, err
		}
		cfg, err := config.Load(path)
		if err != nil {
			return launch{}, err
		}
		return mergeLaunch(&cfg, f), nil
	}
	on := true
	cfg := config.File{Model: res.Model, Tape: res.Tape, Scale: 2, Sound: &on}
	return mergeLaunch(&cfg, f), nil
}

func stayOnPicker(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.HasPrefix(msg, "rom:") || strings.Contains(msg, "this snapshot needs --model 128")
}

func buildSession(l launch, exeDir, work string) (*Session, error) {
	path := l.File
	if path != "" && !filepath.IsAbs(path) && l.Data == nil {
		path = filepath.Join(work, path)
	}
	var (
		name    string
		errText string
		tape    *formats.Tape
		snap    *formats.Snapshot
	)
	if path != "" {
		data := l.Data
		if data == nil {
			if err := checkExt(path); err != nil {
				return nil, err
			}
			var err error
			if data, err = os.ReadFile(path); err != nil {
				return nil, fmt.Errorf("%s: %v", path, err)
			}
		} else {
			path = ""
		}
		var err error
		name, tape, snap, errText, err = loadProgram(l.File, data)
		if err != nil {
			return nil, err
		}
	}
	model := l.Model
	if model == "" {
		model = "48"
	}
	if snap != nil && snap.Machine128 && model == "48" && l.ROM == "" && !l.ModelExplicit {
		model = "128"
	}
	rom, err := resolveROM(model, l.ROM, exeDir, work)
	if err != nil {
		return nil, err
	}
	m := machine.New()
	if err := m.LoadROM(rom); err != nil {
		return nil, err
	}
	if snap != nil && snap.Machine128 && !m.Is128() {
		return nil, fmt.Errorf("this snapshot needs --model 128 and a 32768-byte ROM")
	}
	if snap != nil {
		m.ApplySnapshot(toSnap(*snap))
	} else {
		m.Reset()
		if tape != nil {
			m.ArmTape(toPulses(tape.Edges), tape.ForceLow, !l.NoAutoload)
		}
		if errText != "" {
			m.SetError(errText)
		}
	}
	notice := ""
	if !l.Sound {
		notice = "sound off"
	}
	return &Session{
		Launch:       l,
		Path:         path,
		Mach:         m,
		Scale:        l.Scale,
		Sound:        l.Sound,
		RealtimeTape: l.Realtime,
		NoAutoload:   l.NoAutoload,
		Name:         name,
		Notice:       notice,
		ErrorText:    errText,
	}, nil
}

// checkExt refuses a file the emulator cannot load, by extension.
func checkExt(path string) error {
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".tap", ".tzx", ".sna", ".z80":
		return nil
	default:
		return fmt.Errorf("%s: unknown extension %q (supported: .tap, .tzx, .sna, .z80)", path, ext)
	}
}

// loadProgram decodes a program by the extension of path. A file that
// does not decode is not an error: errText is shown and the machine runs.
func loadProgram(path string, data []byte) (name string, tape *formats.Tape, snap *formats.Snapshot, errText string, err error) {
	if err := checkExt(path); err != nil {
		return "", nil, nil, "", err
	}
	name = filepath.Base(path)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".tap":
		_, t, err := formats.ReadTAP(data)
		if err != nil {
			errText = err.Error()
		} else {
			tape = &t
		}
	case ".tzx":
		t, err := formats.ReadTZX(data)
		if err != nil {
			errText = err.Error()
		} else {
			tape = &t
		}
	case ".sna":
		s, err := formats.ReadSNA(data)
		if err != nil {
			errText = err.Error()
		} else {
			snap = &s
		}
	case ".z80":
		s, err := formats.ReadZ80(data)
		if err != nil {
			errText = err.Error()
		} else {
			snap = &s
		}
	}
	return name, tape, snap, errText, nil
}

func resolveROM(model, flagPath, exeDir, workDir string) ([]byte, error) {
	if flagPath != "" {
		b, err := machine.ReadROM(flagPath)
		if err != nil {
			return nil, err
		}
		want := 16384
		if model == "128" {
			want = 32768
		}
		if len(b) != want {
			return nil, fmt.Errorf("rom: %s is %d bytes, want %d for --model %s", flagPath, len(b), want, model)
		}
		return b, nil
	}
	if model == "128" {
		return findROM(exeDir, workDir, "spec128uk.rom", "128.rom")
	}
	return findROM(exeDir, workDir, "48.rom")
}

func findROM(exeDir, workDir string, names ...string) ([]byte, error) {
	var tried []string
	for _, dir := range []string{exeDir, workDir} {
		for _, name := range names {
			p := filepath.Join(dir, "rom", name)
			tried = append(tried, p)
			st, err := os.Stat(p)
			if err != nil || st.IsDir() {
				continue
			}
			return machine.ReadROM(p)
		}
	}
	return nil, fmt.Errorf("rom: not found at %s", strings.Join(tried, " or "))
}

func toSnap(s formats.Snapshot) machine.Snap {
	r := s.Regs
	return machine.Snap{
		A: r.A, F: r.F,
		B: r.B, C: r.C,
		D: r.D, E: r.E,
		H: r.H, L: r.L,
		Ap: r.Ap, Fp: r.Fp,
		Bp: r.Bp, Cp: r.Cp,
		Dp: r.Dp, Ep: r.Ep,
		Hp: r.Hp, Lp: r.Lp,
		IX: r.IX, IY: r.IY,
		SP: r.SP, PC: r.PC,
		I: r.I, R: r.R,
		IFF1: r.IFF1, IFF2: r.IFF2,
		IM:       r.IM,
		Border:   s.Border,
		RAM:      s.RAM,
		PopPC:    s.PopPC,
		MEMPTR:   0,
		Halted:   false,
		Model128: s.Machine128,
		Banks:    s.Banks,
		Page:     s.Page,
		AYSel:    s.AYSel,
		AYReg:    s.AYReg,
	}
}

// fromSnap is toSnap in reverse, for writing a save state.
func fromSnap(s machine.Snap) formats.Snapshot {
	return formats.Snapshot{
		Regs: formats.Regs{
			A: s.A, F: s.F,
			B: s.B, C: s.C,
			D: s.D, E: s.E,
			H: s.H, L: s.L,
			Ap: s.Ap, Fp: s.Fp,
			Bp: s.Bp, Cp: s.Cp,
			Dp: s.Dp, Ep: s.Ep,
			Hp: s.Hp, Lp: s.Lp,
			IX: s.IX, IY: s.IY,
			SP: s.SP, PC: s.PC,
			I: s.I, R: s.R,
			IFF1: s.IFF1, IFF2: s.IFF2,
			IM: s.IM,
		},
		RAM:        s.RAM,
		Border:     s.Border,
		Machine128: s.Model128,
		Banks:      s.Banks,
		Page:       s.Page,
		AYSel:      s.AYSel,
		AYReg:      s.AYReg,
	}
}

func toPulses(in []formats.Pulse) []machine.Pulse {
	out := make([]machine.Pulse, len(in))
	for i, p := range in {
		out[i] = machine.Pulse{Level: p.Level, Dur: p.Dur}
	}
	return out
}
