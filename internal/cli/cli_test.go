package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// capture runs fn with os.Stdout and os.Stderr redirected to a buffer
// and returns (exitCode, combinedOutput). Streams are restored even
// if fn panics.
func capture(t *testing.T, fn func() int) (int, string) {
	t.Helper()

	origOut, origErr := os.Stdout, os.Stderr
	t.Cleanup(func() {
		os.Stdout = origOut
		os.Stderr = origErr
	})

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	t.Cleanup(func() { r.Close() })

	os.Stdout = w
	os.Stderr = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	code := fn()
	_ = w.Close()
	<-done
	return code, buf.String()
}

func TestRunVersionFlagWritesToStdout(t *testing.T) {
	code, out := capture(t, func() int {
		return Run([]string{"--version"})
	})
	if code != 0 {
		t.Fatalf("expected exit 0 for --version, got %d", code)
	}
	if !strings.Contains(out, "dia version") {
		t.Fatalf("expected output to contain 'dia version', got: %q", out)
	}
}

func TestRunShortVersionFlag(t *testing.T) {
	code, out := capture(t, func() int {
		return Run([]string{"-V"})
	})
	if code != 0 {
		t.Fatalf("expected exit 0 for -V, got %d", code)
	}
	if !strings.Contains(out, "dia version") {
		t.Fatalf("expected output to contain 'dia version', got: %q", out)
	}
}

func TestRunHelpWritesHelp(t *testing.T) {
	code, out := capture(t, func() int {
		return Run([]string{"--help"})
	})
	if code != 0 {
		t.Fatalf("expected exit 0 for --help, got %d", code)
	}
	if !strings.Contains(out, "dia") {
		t.Fatalf("expected help to mention dia, got: %q", out)
	}
}

func TestRunWithNoArgsShowsHelp(t *testing.T) {
	code, out := capture(t, func() int {
		return Run([]string{})
	})
	if code != 0 {
		t.Fatalf("expected exit 0 for no args, got %d", code)
	}
	if !strings.Contains(out, "dia") {
		t.Fatalf("expected help to mention dia, got: %q", out)
	}
}
