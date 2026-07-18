package config

import (
	"strings"
	"testing"
)

func TestParseHooks(t *testing.T) {
	w, err := Parse([]byte(`name: hooked
apps:
  - type: local
    cmd: app
hooks:
  pre_start:
    - docker compose up -d
    - ./scripts/seed.sh
  post_stop:
    - docker compose down
`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Hooks == nil {
		t.Fatal("hooks not parsed")
	}
	if len(w.Hooks.PreStart) != 2 || w.Hooks.PreStart[0] != "docker compose up -d" {
		t.Errorf("pre_start: %+v", w.Hooks.PreStart)
	}
	if len(w.Hooks.PostStop) != 1 || w.Hooks.PostStop[0] != "docker compose down" {
		t.Errorf("post_stop: %+v", w.Hooks.PostStop)
	}
	if len(w.Hooks.PostStart) != 0 || len(w.Hooks.PreStop) != 0 {
		t.Errorf("unset phases should be empty: %+v", w.Hooks)
	}
}

func TestParseHooksOmittedIsNil(t *testing.T) {
	w, err := Parse([]byte("name: plain\napps:\n  - type: local\n    cmd: app\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Hooks != nil {
		t.Errorf("hooks should be nil when absent, got %+v", w.Hooks)
	}
}

func TestParseHooksRejectsBlankCommand(t *testing.T) {
	_, err := Parse([]byte(`name: hooked
apps:
  - type: local
    cmd: app
hooks:
  pre_start:
    - "  "
`))
	if err == nil {
		t.Fatal("expected an error for a blank hook command")
	}
	if !strings.Contains(err.Error(), "workspace.hooks.pre_start[0]") {
		t.Errorf("error should name the exact path, got: %v", err)
	}
}

func TestParseHooksRejectsUnknownPhase(t *testing.T) {
	_, err := Parse([]byte(`name: hooked
apps:
  - type: local
    cmd: app
hooks:
  during_start:
    - nope
`))
	if err == nil {
		t.Fatal("expected an error for an unknown hook phase")
	}
}

// The `wait` and `open` app fields were declared in the schema but
// never read by the runtime or registry. They are gone; because the
// loader uses KnownFields(true), a config still setting them now
// fails loudly instead of silently doing nothing.
func TestParseRejectsRemovedAppFields(t *testing.T) {
	for _, field := range []string{"wait: true", "open: true"} {
		_, err := Parse([]byte("name: legacy\napps:\n  - type: local\n    cmd: app\n    " + field + "\n"))
		if err == nil {
			t.Errorf("%s: expected an error for a removed field", field)
		}
	}
}

func TestHooksPhasesNilReceiver(t *testing.T) {
	var h *Hooks
	if got := h.Phases(); got != nil {
		t.Errorf("Phases() on a nil Hooks = %v, want nil", got)
	}
}
