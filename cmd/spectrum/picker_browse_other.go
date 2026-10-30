//go:build !darwin

package main

import "errors"

func browseFile() (string, error) {
	return "", errors.New("browse is not available on this system")
}
