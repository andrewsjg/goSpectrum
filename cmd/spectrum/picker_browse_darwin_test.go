//go:build darwin

package main

import "testing"

func TestBrowseCancelMessage(t *testing.T) {
	if !browseCanceled("execution error: User canceled. (-128)") {
		t.Fatal("cancel")
	}
	if !browseCanceled("User cancelled.") {
		t.Fatal("cancelled")
	}
	if browseCanceled("osascript: syntax error") {
		t.Fatal("syntax is not a cancel")
	}
}
