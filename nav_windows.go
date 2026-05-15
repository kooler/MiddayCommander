//go:build windows

package main

import (
	"errors"
	"os"
)

func openControllingTTY() (*os.File, error) {
	return nil, errors.New("-r is not supported on Windows")
}
