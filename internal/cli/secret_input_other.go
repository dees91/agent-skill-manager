//go:build !darwin && !linux

package cli

import (
	"fmt"
	"io"
	"os"
)

func readTTYSecret(*os.File, io.Writer) (string, error) {
	return "", fmt.Errorf("standard input is not a terminal; use --key-stdin")
}
