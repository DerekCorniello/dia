package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidationError describes a single field-level problem with a workspace.
// Path is a dotted/bracket path like "workspace.apps[2].cmd".
type ValidationError struct {
	Path string
	Msg  string
}

func (e ValidationError) Error() string {
	return e.Path + ": " + e.Msg
}

// ValidationErrors aggregates multiple ValidationError values so the
// caller can see every problem with a config in one pass.
type ValidationErrors []ValidationError

func (es ValidationErrors) Error() string {
	if len(es) == 0 {
		return ""
	}
	parts := make([]string, len(es))
	for i, e := range es {
		parts[i] = e.Error()
	}
	return "invalid workspace:\n  " + strings.Join(parts, "\n  ")
}

func (es ValidationErrors) Is(target error) bool {
	_, ok := target.(ValidationErrors)
	return ok
}

// Load reads a YAML file from path, unmarshals it, validates the result,
// and returns the workspace.
func Load(path string) (*Workspace, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return Parse(data)
}

// Parse unmarshals YAML bytes, validates the result, and returns the
// workspace. Used by Load and by tests.
func Parse(data []byte) (*Workspace, error) {
	var w Workspace
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&w); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if err := Validate(&w); err != nil {
		return nil, err
	}
	return &w, nil
}

// Validate checks a workspace against the schema. The returned error is
// either nil or a ValidationErrors value listing every problem.
func Validate(w *Workspace) error {
	if w == nil {
		return ValidationErrors{{Path: "workspace", Msg: "nil"}}
	}
	var errs ValidationErrors

	if w.Version == 0 {
		// Default to current; treat unset as 1 so existing
		// configs written before versioning don't break.
		w.Version = SchemaVersion
	}
	if w.Version > SchemaVersion {
		errs = append(errs, ValidationError{
			Path: "workspace.version",
			Msg:  fmt.Sprintf("config version %d is newer than dia's max supported version %d", w.Version, SchemaVersion),
		})
	}

	if w.Name == "" {
		errs = append(errs, ValidationError{Path: "workspace.name", Msg: "required"})
	} else if !validName(w.Name) {
		errs = append(errs, ValidationError{
			Path: "workspace.name",
			Msg:  "must match ^[a-z0-9][a-z0-9-]*$",
		})
	}

	for i := range w.Apps {
		validateApp(&w.Apps[i], fmt.Sprintf("workspace.apps[%d]", i), &errs)
	}

	validateHooks(w.Hooks, &errs)

	if len(errs) == 0 {
		return nil
	}
	return errs
}

// validateHooks rejects blank hook commands. A blank entry is almost
// always a YAML mistake (a stray "-" or an empty quoted string), and
// letting it through means a confusing "cmd is empty" at start time
// instead of a precise path here.
func validateHooks(h *Hooks, errs *ValidationErrors) {
	for _, phase := range h.Phases() {
		for i, cmd := range phase.Cmds {
			if strings.TrimSpace(cmd) == "" {
				*errs = append(*errs, ValidationError{
					Path: fmt.Sprintf("workspace.hooks.%s[%d]", phase.Name, i),
					Msg:  "must not be empty",
				})
			}
		}
	}
}

func validateApp(a *App, prefix string, errs *ValidationErrors) {
	switch a.Type {
	case "editor", "terminal":
		if a.Cmd == "" {
			*errs = append(*errs, ValidationError{
				Path: prefix + ".cmd",
				Msg:  fmt.Sprintf("required for type %q", a.Type),
			})
		}
	case "service", "custom", "local":
		if a.Cmd == "" {
			*errs = append(*errs, ValidationError{
				Path: prefix + ".cmd",
				Msg:  fmt.Sprintf("required for type %q", a.Type),
			})
		}
	case "open":
		// `open` is the general URL-in-default-handler type;
		// mailto:, file://, ssh://, custom schemes are all fine.
		if a.Url == "" {
			*errs = append(*errs, ValidationError{
				Path: prefix + ".url",
				Msg:  "required for type \"open\"",
			})
		}
	case "browser":
		// The runtime handler (resolveBrowser) only ever opens a.Url;
		// it never reads a.Cmd. Require url unconditionally so an
		// invalid cmd-only entry is rejected here instead of passing
		// validation and --dry-run, then always failing at a real
		// `dia start` with "url is required".
		if a.Url == "" {
			*errs = append(*errs, ValidationError{
				Path: prefix + ".url",
				Msg:  "required for type \"browser\"",
			})
		} else if !strings.HasPrefix(a.Url, "http://") && !strings.HasPrefix(a.Url, "https://") {
			*errs = append(*errs, ValidationError{
				Path: prefix + ".url",
				Msg:  "must start with http:// or https://",
			})
		}
	case "gh":
		// `gh` is a thin wrapper around the gh CLI. The first
		// positional (cmd) is the gh subcommand, the rest are
		// its arguments.
		if a.Cmd == "" {
			*errs = append(*errs, ValidationError{
				Path: prefix + ".cmd",
				Msg:  "required for type \"gh\" (the gh subcommand, e.g. \"pr\")",
			})
		}
	case "gh:pr", "gh:issue", "gh:checkout":
		// Shorthand: the subcommand is baked into the type. No
		// required fields; args are passed through.
	case "gh:repo-clone":
		if a.Url == "" {
			*errs = append(*errs, ValidationError{
				Path: prefix + ".url",
				Msg:  "required for type \"gh:repo-clone\"",
			})
		}
	default:
		// Unknown types are accepted by the loader but the
		// runtime will refuse to start them. Validation here
		// stays lenient so the user can see every other
		// problem in the workspace at once.
	}

	if a.Type == "" && a.Cmd == "" && a.Url == "" {
		*errs = append(*errs, ValidationError{
			Path: prefix,
			Msg:  "must have type, cmd, or url",
		})
	}
}

// ValidateName returns an error if s is not a valid workspace
// name. Names must be non-empty, alphanumeric (a-z, A-Z, 0-9),
// with hyphens and underscores allowed anywhere.
func ValidateName(s string) error {
	if !validName(s) {
		return fmt.Errorf("invalid workspace name %q: must be alphanumeric with hyphens or underscores", s)
	}
	return nil
}

func validName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

// IsValidationError reports whether err is (or wraps) a ValidationErrors.
func IsValidationError(err error) bool {
	var ve ValidationErrors
	return errors.As(err, &ve)
}
