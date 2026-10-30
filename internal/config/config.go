// Package config stores named launcher choices as JSON.
// A file lives in the user config directory under goSpectrum, or in a
// directory the caller supplies. The emulator does not import this package
// from the machine.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// File is one saved start: a 48K or 128K machine and an optional program.
// Sound is a pointer so a file that omits it stays on. Scale 0 means 2.
type File struct {
	Model        string `json:"model"`
	Tape         string `json:"tape,omitempty"`
	ROM          string `json:"rom,omitempty"`
	Scale        int    `json:"scale,omitempty"`
	Sound        *bool  `json:"sound,omitempty"`
	RealtimeTape bool   `json:"realtimeTape,omitempty"`
	NoAutoload   bool   `json:"noAutoload,omitempty"`
}

// SoundOn reports the saved audio switch. A missing field is on.
func (f File) SoundOn() bool {
	if f.Sound == nil {
		return true
	}
	return *f.Sound
}

// ScaleOr returns the saved scale, or def when the file left it unset.
func (f File) ScaleOr(def int) int {
	if f.Scale < 1 {
		return def
	}
	return f.Scale
}

// Dir is override, or the user config directory plus goSpectrum.
func Dir(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "goSpectrum"), nil
}

var errBadName = errors.New("config: name must be letters, numbers, spaces, '.', '_' or '-'")

// ValidName trims space, strips one trailing .json, and checks the rules.
// The result is the name without the extension. The file on disk is that
// name plus .json.
func ValidName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if len(name) >= 5 && strings.EqualFold(name[len(name)-5:], ".json") {
		name = strings.TrimSpace(name[:len(name)-5])
	}
	if name == "" || name == "." || name == ".." || len([]rune(name)) > 40 {
		return "", errBadName
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == ' ' || r == '.' || r == '_' || r == '-':
		default:
			return "", errBadName
		}
	}
	return name, nil
}

// Resolve finds a config. A name is looked up as <name>.json in dir.
// An argument whose base differs from itself is a path and must exist.
func Resolve(dir, arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", errBadName
	}
	if filepath.Base(arg) != arg {
		st, err := os.Stat(arg)
		if err != nil || st.IsDir() {
			return "", fmt.Errorf("config: %q not found in %s", filepath.Base(arg), filepath.Dir(arg))
		}
		return arg, nil
	}
	name, err := ValidName(arg)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, name+".json")
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return "", fmt.Errorf("config: %q not found in %s", arg, dir)
	}
	return path, nil
}

// Load reads a config file. Model must be 48 or 128.
func Load(path string) (File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	var f File
	if err := json.Unmarshal(raw, &f); err != nil {
		return File{}, fmt.Errorf("config: %s", err.Error())
	}
	if f.Model != "48" && f.Model != "128" {
		return File{}, fmt.Errorf("config: model must be 48 or 128")
	}
	return f, nil
}

// Save writes <name>.json in dir, creating dir when it is missing.
// Sound is always written. An unset scale is stored as 2.
func Save(dir, name string, f File) error {
	canon, err := ValidName(name)
	if err != nil {
		return err
	}
	if f.Model != "48" && f.Model != "128" {
		return fmt.Errorf("config: model must be 48 or 128")
	}
	on := true
	if f.Sound != nil {
		on = *f.Sound
	}
	f.Sound = &on
	if f.Scale < 1 {
		f.Scale = 2
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(filepath.Join(dir, canon+".json"), raw, 0o644)
}

// List returns config names without the .json suffix, in order.
// A missing directory is an empty list.
func List(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		canon, err := ValidName(e.Name())
		if err != nil {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		names = append(names, canon)
	}
	sort.Strings(names)
	return names, nil
}
