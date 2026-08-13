package contextbudget

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const maxDiagnosticOutput = 8 << 20

type commandRunner interface {
	Run(ctx context.Context, dir, binary string, args ...string) ([]byte, error)
}

type execRunner struct {
	home string
}

func (runner execRunner) Run(ctx context.Context, dir, binary string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = dir
	command.Env = environmentWithHome(os.Environ(), runner.home)
	stdout := &limitedBuffer{limit: maxDiagnosticOutput}
	stderr := &limitedBuffer{limit: 16 << 10}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%s %v: %w: %s", binary, args, err, stderr.String())
	}
	if stdout.overflow {
		return nil, fmt.Errorf("%s diagnostic output exceeded %d bytes", binary, maxDiagnosticOutput)
	}
	return stdout.Bytes(), nil
}

func environmentWithHome(environment []string, home string) []string {
	result := make([]string, 0, len(environment)+1)
	for _, item := range environment {
		name, _, _ := strings.Cut(item, "=")
		if name == "HOME" || name == "CODEX_HOME" || name == "CLAUDE_CONFIG_DIR" {
			continue
		}
		result = append(result, item)
	}
	return append(result, "HOME="+home)
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	original := len(data)
	remaining := b.limit - b.Len()
	if remaining <= 0 {
		b.overflow = true
		return original, nil
	}
	if len(data) > remaining {
		data = data[:remaining]
		b.overflow = true
	}
	_, _ = b.Buffer.Write(data)
	return original, nil
}
