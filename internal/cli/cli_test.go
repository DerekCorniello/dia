package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// runWith runs args with the given env overrides. It returns
// (exitCode, stdout, stderr). The XDG vars default to temp dirs so
// the test does not touch the user's real state or config; pass
// values in env to override.
//
// env values of "" unset the variable. The test is restored on
// Cleanup.
func runWith(t *testing.T, env map[string]string, args ...string) (int, string, string) {
	t.Helper()
	baseEnv := map[string]string{
		"XDG_STATE_HOME":  t.TempDir(),
		"XDG_CONFIG_HOME": t.TempDir(),
		"HOME":            t.TempDir(),
		"PATH":            os.Getenv("PATH"),
	}
	for k, v := range env {
		if v == "" {
			delete(baseEnv, k)
		} else {
			baseEnv[k] = v
		}
	}
	for k, v := range baseEnv {
		t.Setenv(k, v)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	t.Cleanup(func() {
		os.Stdout = oldOut
		os.Stderr = oldErr
	})
	outR, outW, err := os.Pipe()
	errR, errW, err2 := os.Pipe()
	if err != nil || err2 != nil {
		t.Fatalf("pipe: %v %v", err, err2)
	}
	os.Stdout = outW
	os.Stderr = errW
	var outBuf, errBuf bytes.Buffer
	outDone := make(chan struct{})
	errDone := make(chan struct{})
	go func() { _, _ = io.Copy(&outBuf, outR); close(outDone) }()
	go func() { _, _ = io.Copy(&errBuf, errR); close(errDone) }()

	code := Run(args)

	_ = outW.Close()
	_ = errW.Close()
	<-outDone
	<-errDone
	return code, outBuf.String(), errBuf.String()
}

func TestRun_Help(t *testing.T) {
	code, out, _ := runWith(t, nil, "--help")
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "Available Commands") {
		t.Errorf("expected help to list commands, got: %q", out)
	}
}

func TestRun_Version(t *testing.T) {
	code, out, _ := runWith(t, nil, "--version")
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "dia version") {
		t.Errorf("expected 'dia version', got: %q", out)
	}
}

func TestList_Empty(t *testing.T) {
	code, out, _ := runWith(t, nil, "list")
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "no workspaces found") {
		t.Errorf("expected 'no workspaces found', got: %q", out)
	}
}

func TestList_WithWorkspaces(t *testing.T) {
	cfgDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cfgDir, "dia", "workspaces"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "dia", "workspaces", "alpha.yaml"),
		[]byte("version: 1\nname: alpha\napps:\n  - type: local\n    cmd: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runWith(t, map[string]string{"XDG_CONFIG_HOME": cfgDir}, "list")
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "alpha") {
		t.Errorf("expected 'alpha' in output, got: %q", out)
	}
}

func TestList_JSON(t *testing.T) {
	cfgDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cfgDir, "dia", "workspaces"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "dia", "workspaces", "beta.yaml"),
		[]byte("version: 1\nname: beta\napps:\n  - type: local\n    cmd: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runWith(t, map[string]string{"XDG_CONFIG_HOME": cfgDir}, "--json", "list")
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if len(rows) != 1 || rows[0]["name"] != "beta" {
		t.Errorf("rows = %v", rows)
	}
}

func TestNew_Global(t *testing.T) {
	cfgDir := t.TempDir()
	code, out, _ := runWith(t, map[string]string{"XDG_CONFIG_HOME": cfgDir}, "new", "fresh")
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	want := filepath.Join(cfgDir, "dia", "workspaces", "fresh.yaml")
	if !strings.Contains(out, want) {
		t.Errorf("expected %q in output, got: %q", want, out)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestNew_Local(t *testing.T) {
	cwd := t.TempDir()
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldCwd) })
	code, out, _ := runWith(t, nil, "new", "--local", "localws")
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, ".dia.yaml") {
		t.Errorf("expected '.dia.yaml' in output, got: %q", out)
	}
	if _, err := os.Stat(filepath.Join(cwd, ".dia.yaml")); err != nil {
		t.Errorf(".dia.yaml not created: %v", err)
	}
}

func TestNew_AlreadyExists(t *testing.T) {
	cfgDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cfgDir, "dia", "workspaces"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfgDir, "dia", "workspaces", "dup.yaml")
	if err := os.WriteFile(path, []byte("exists"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := runWith(t, map[string]string{"XDG_CONFIG_HOME": cfgDir}, "new", "dup")
	if code == 0 {
		t.Errorf("expected non-zero exit, got 0")
	}
	if !strings.Contains(errOut, "already exists") {
		t.Errorf("expected 'already exists' in stderr, got: %q", errOut)
	}
}

func TestStart_NotFound(t *testing.T) {
	code, _, errOut := runWith(t, nil, "start", "ghost")
	if code != ExitNotFound {
		t.Errorf("exit = %d, want %d", code, ExitNotFound)
	}
	if !strings.Contains(errOut, "ghost") {
		t.Errorf("expected 'ghost' in stderr, got: %q", errOut)
	}
}

func TestStart_InvalidArgs(t *testing.T) {
	code, _, _ := runWith(t, nil, "start")
	if code != ExitUsage {
		t.Errorf("exit = %d, want %d", code, ExitUsage)
	}
}

func TestStatus_Empty(t *testing.T) {
	code, out, _ := runWith(t, nil, "status")
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "no instances") {
		t.Errorf("expected 'no instances', got: %q", out)
	}
}

func TestStop_NotRunning(t *testing.T) {
	code, _, errOut := runWith(t, nil, "stop", "nope")
	if code != ExitNotFound {
		t.Errorf("exit = %d, want %d", code, ExitNotFound)
	}
	if !strings.Contains(errOut, "nope") {
		t.Errorf("expected 'nope' in stderr, got: %q", errOut)
	}
}

func TestReconcile_Empty(t *testing.T) {
	code, out, _ := runWith(t, nil, "reconcile")
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "reconciled 0") {
		t.Errorf("expected 'reconciled 0', got: %q", out)
	}
}

func TestReconcile_JSON(t *testing.T) {
	code, out, _ := runWith(t, nil, "--json", "reconcile")
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	var m map[string]int
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if m["reconciled"] != 0 || m["remaining"] != 0 {
		t.Errorf("unexpected counts: %v", m)
	}
}

func TestDoctor(t *testing.T) {
	code, out, _ := runWith(t, nil, "doctor")
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "platform") {
		t.Errorf("expected 'platform' in output, got: %q", out)
	}
}

func TestPlugins_Empty(t *testing.T) {
	code, out, _ := runWith(t, map[string]string{"PATH": ""}, "plugins")
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "no dia-* plugins") {
		t.Errorf("expected 'no dia-* plugins', got: %q", out)
	}
}

func TestPlugins_WithFixture(t *testing.T) {
	plugDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(plugDir, "dia-fake"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runWith(t, map[string]string{"PATH": plugDir}, "plugins")
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "dia-fake") {
		t.Errorf("expected 'dia-fake' in output, got: %q", out)
	}
}

func TestStartStopRoundtrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command not available on Windows")
	}
	stateDir := t.TempDir()
	cfgDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cfgDir, "dia", "workspaces"), 0o755); err != nil {
		t.Fatal(err)
	}
	yamlPath := filepath.Join(cfgDir, "dia", "workspaces", "rt.yaml")
	if err := os.WriteFile(yamlPath, []byte(`version: 1
name: rt
apps:
  - type: local
    cmd: sleep 30
`), 0o644); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"XDG_STATE_HOME":  stateDir,
		"XDG_CONFIG_HOME": cfgDir,
	}

	code, startOut, _ := runWith(t, env, "start", "rt")
	if code != 0 {
		t.Fatalf("start exit = %d, want 0; out: %s", code, startOut)
	}
	if !strings.Contains(startOut, "started rt") {
		t.Errorf("expected 'started rt', got: %q", startOut)
	}

	code, statOut, _ := runWith(t, env, "status")
	if code != 0 {
		t.Errorf("status exit = %d", code)
	}
	if !strings.Contains(statOut, "rt") || !strings.Contains(statOut, "running") {
		t.Errorf("expected running rt, got: %q", statOut)
	}

	code, stopOut, _ := runWith(t, env, "stop", "rt")
	if code != 0 {
		t.Errorf("stop exit = %d", code)
	}
	if !strings.Contains(stopOut, "stopped rt") {
		t.Errorf("expected 'stopped rt', got: %q", stopOut)
	}

	code, after, _ := runWith(t, env, "status")
	if code != 0 {
		t.Errorf("status exit = %d", code)
	}
	if !strings.Contains(after, "stopped") {
		t.Errorf("expected 'stopped' in status, got: %q", after)
	}
}

func TestStartStopRoundtrip_JSON(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command not available on Windows")
	}
	stateDir := t.TempDir()
	cfgDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cfgDir, "dia", "workspaces"), 0o755); err != nil {
		t.Fatal(err)
	}
	yamlPath := filepath.Join(cfgDir, "dia", "workspaces", "rt2.yaml")
	if err := os.WriteFile(yamlPath, []byte(`version: 1
name: rt2
apps:
  - type: local
    cmd: sleep 30
`), 0o644); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"XDG_STATE_HOME":  stateDir,
		"XDG_CONFIG_HOME": cfgDir,
	}

	code, startOut, _ := runWith(t, env, "--json", "start", "rt2")
	if code != 0 {
		t.Fatalf("start exit = %d, out: %s", code, startOut)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(startOut), &m); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, startOut)
	}
	if m["status"] != "running" {
		t.Errorf("status = %v, want running", m["status"])
	}
	id, ok := m["id"].(string)
	if !ok || id == "" {
		t.Errorf("id missing or wrong type: %v", m["id"])
	}

	code, _, _ = runWith(t, env, "--json", "stop", "rt2")
	if code != 0 {
		t.Errorf("stop exit = %d", code)
	}
}

func TestStopAll_NoArgs(t *testing.T) {
	code, out, _ := runWith(t, nil, "stop", "--all")
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "stopped all") {
		t.Errorf("expected 'stopped all', got: %q", out)
	}
}
