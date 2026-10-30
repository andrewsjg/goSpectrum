package machine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadROM(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "48.rom")
	body := make([]byte, 16384)
	body[0] = 0xF3
	if err := os.WriteFile(good, body, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadROM(good)
	if err != nil || len(got) != 16384 || got[0] != 0xF3 {
		t.Fatalf("good rom: %v len %d", err, len(got))
	}
	bad := filepath.Join(dir, "short.rom")
	if err := os.WriteFile(bad, body[:100], 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = ReadROM(bad)
	if err == nil || got != nil || !strings.Contains(err.Error(), bad) {
		t.Fatalf("short: %v %v", got, err)
	}
	missing := filepath.Join(dir, "missing.rom")
	got, err = ReadROM(missing)
	if err == nil || got != nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("missing: %v %v", got, err)
	}
}

func TestCharset(t *testing.T) {
	rom48 := make([]byte, 16384)
	rom48[0x3D00] = 0xAA
	if cs := Charset(rom48); len(cs) != 768 || cs[0] != 0xAA {
		t.Fatal("48K charset")
	}
	rom128 := make([]byte, 32768)
	rom128[16384+0x3D00+767] = 0x55
	if cs := Charset(rom128); len(cs) != 768 || cs[767] != 0x55 {
		t.Fatal("128K charset is not from the BASIC ROM")
	}
	if Charset(make([]byte, 100)) != nil {
		t.Fatal("bad length")
	}
}
