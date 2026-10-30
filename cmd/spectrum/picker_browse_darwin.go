//go:build darwin

package main

import (
	"errors"
	osexec "os/exec"
	"strings"
)

// browseFile asks the Mac to choose one file. Cancel returns an empty path.
func browseFile() (string, error) {
	const script = `POSIX path of (choose file with prompt "Choose a tape or snapshot")`
	out, err := osexec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if browseCanceled(msg) {
			return "", nil
		}
		if msg == "" {
			return "", err
		}
		return "", errors.New(msg)
	}
	return strings.TrimSpace(string(out)), nil
}

func browseCanceled(msg string) bool {
	low := strings.ToLower(msg)
	return strings.Contains(msg, "-128") || strings.Contains(low, "user canceled") || strings.Contains(low, "user cancelled")
}
