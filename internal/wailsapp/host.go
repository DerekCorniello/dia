package wailsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"
)

// wailsHost implements plugins.HostAPI by delegating to the wails
// App's bound methods. It bridges the strongly-typed wails surface
// to the loosely-typed interface plugins expect.
type wailsHost struct {
	app *App
}

func (h *wailsHost) ListWorkspaces(ctx context.Context) ([]any, error) {
	infos, err := h.app.ListWorkspaces()
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(infos))
	for _, w := range infos {
		m, err := marshalAny(w)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func (h *wailsHost) GetWorkspace(ctx context.Context, name string) (any, error) {
	d, err := h.app.GetWorkspace(name)
	if err != nil {
		return nil, err
	}
	return marshalAny(d)
}

func (h *wailsHost) StartWorkspace(ctx context.Context, name string) (any, error) {
	if err := h.app.StartWorkspace(name); err != nil {
		return nil, err
	}
	ws, _, err := h.app.findWorkspace(name)
	if err != nil {
		return nil, err
	}
	return marshalAny(ws)
}

func (h *wailsHost) ListInstances(ctx context.Context) ([]any, error) {
	insts := h.app.ListInstances()
	out := make([]any, 0, len(insts))
	for _, i := range insts {
		m, err := marshalAny(i)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// marshalAny round-trips a value through JSON to convert typed structs
// to map[string]any for the Wails JS bridge.
func marshalAny(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	var m any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return m, nil
}

func (h *wailsHost) StopInstance(ctx context.Context, id string) error {
	return h.app.StopInstance(id)
}

func (h *wailsHost) StopAll(ctx context.Context) (int, error) {
	return h.app.StopAll()
}

func (h *wailsHost) Doctor(ctx context.Context) ([]any, error) {
	checks := h.app.Doctor()
	out := make([]any, 0, len(checks))
	for _, c := range checks {
		out = append(out, c)
	}
	return out, nil
}

func (h *wailsHost) Paths(ctx context.Context) (any, error) {
	return marshalAny(h.app.Paths())
}

func (h *wailsHost) GetTheme(ctx context.Context) (string, error) {
	return h.app.GetTheme(), nil
}

func (h *wailsHost) SetTheme(ctx context.Context, name string) error {
	return h.app.SetTheme(name)
}

func (h *wailsHost) ListCustomThemes(ctx context.Context) ([]any, error) {
	themes := h.app.ListCustomThemes()
	out := make([]any, 0, len(themes))
	for _, t := range themes {
		out = append(out, t)
	}
	return out, nil
}

func (h *wailsHost) SetCustomTheme(ctx context.Context, info any) error {
	b, err := json.Marshal(info)
	if err != nil {
		return err
	}
	var ci CustomThemeInfo
	if err := json.Unmarshal(b, &ci); err != nil {
		return err
	}
	return h.app.SetCustomTheme(ci)
}

func (h *wailsHost) DeleteCustomTheme(ctx context.Context, name string) error {
	return h.app.DeleteCustomTheme(name)
}

func (h *wailsHost) NewWorkspace(ctx context.Context, name string) (string, error) {
	return h.app.NewWorkspace(name, "")
}

func (h *wailsHost) Exec(ctx context.Context, cmd string, args []string) (string, error) {
	out, err := exec.CommandContext(ctx, cmd, args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s: %w", string(out), err)
	}
	return string(out), nil
}

func (h *wailsHost) Fetch(ctx context.Context, url string, opts map[string]any) (any, error) {
	method := "GET"
	var body io.Reader
	if m, ok := opts["method"].(string); ok && m != "" {
		method = m
	}
	if b, ok := opts["body"].(string); ok && b != "" {
		body = bytes.NewBufferString(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if headers, ok := opts["headers"].(map[string]any); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, vs)
			}
		}
	}
	if timeoutSec, ok := opts["timeout"].(float64); ok && timeoutSec > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return string(data), fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err == nil {
		return parsed, nil
	}
	return string(data), nil
}
