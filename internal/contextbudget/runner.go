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
	if !allowedDiagnostic(args) {
		return nil, fmt.Errorf("unsupported provider diagnostic arguments: %v", args)
	}
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
	result := make([]string, 0, 8)
	for _, item := range environment {
		name, _, _ := strings.Cut(item, "=")
		if !allowedEnvironmentVariable(name) {
			continue
		}
		result = append(result, item)
	}
	return append(result, "HOME="+home)
}

func allowedEnvironmentVariable(name string) bool {
	switch name {
	case "PATH", "TMPDIR", "LANG", "TERM", "NO_COLOR":
		return true
	default:
		return strings.HasPrefix(name, "LC_")
	}
}

func allowedDiagnostic(args []string) bool {
	joined := strings.Join(args, "\x00")
	return joined == "debug\x00prompt-input" ||
		joined == "debug\x00prompt-input\x00-c\x00model_context_window=100000000" ||
		joined == "plugin\x00list\x00--json"
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
