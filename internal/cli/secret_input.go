package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

func backgroundContext() context.Context {
	return context.Background()
}

func readHiddenSecret(stdin io.Reader, stderr io.Writer, keyStdin bool) (string, error) {
	if keyStdin {
		return readSecretLine(stdin)
	}
	file, ok := stdin.(*os.File)
	if !ok {
		return "", fmt.Errorf("standard input is not a terminal; use --key-stdin")
	}
	return readTTYSecret(file, stderr)
}

func readSecretLine(stdin io.Reader) (string, error) {
	if stdin == nil {
		return "", fmt.Errorf("standard input is not a terminal; use --key-stdin")
	}
	reader := bufio.NewReader(stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read API key")
	}
	secret := strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(secret) == "" {
		return "", fmt.Errorf("API key is required")
	}
	return secret, nil
}
