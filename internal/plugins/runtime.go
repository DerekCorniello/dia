package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dop251/goja"
	"path/filepath"
	"sync"
	"time"
)

const pluginCallTimeout = 30 * time.Second

type Runtime struct {
	mu        sync.Mutex
	rt        *goja.Runtime
	manifest  *Manifest
	entry     string
	host      HostAPI
	bridge    *Bridge
	exited    bool
	exitError error
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewRuntime(manifest *Manifest, pluginDir string, host HostAPI, grants []string, cfg map[string]any) (*Runtime, error) {
	if manifest == nil {
		return nil, errors.New("manifest is nil")
	}
	if host == nil {
		return nil, errors.New("host is nil")
	}
	rt := goja.New()
	entry := filepath.Clean(filepath.Join(pluginDir, manifest.Entry))
	bridge := NewBridge(rt, manifest.Capabilities, grants, host, cfg)
	rt.Set("dia", bridge.DiaObject())
	rt.Set("require", bridge.NewRequire(pluginDir))
	rt.Set("__pluginDir", pluginDir)
	ctx, cancel := context.WithCancel(context.Background())
	return &Runtime{
		rt:       rt,
		manifest: manifest,
		entry:    entry,
		host:     host,
		bridge:   bridge,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}
func (r *Runtime) Load() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.exited {
		return r.exitError
	}
	src, err := readFileLimited(r.entry)
	if err != nil {
		return fmt.Errorf("load %s: %w", r.entry, err)
	}
	program, err := goja.Compile(r.entry, src, true)
	if err != nil {
		return fmt.Errorf("compile %s: %w", r.entry, err)
	}
	module := r.rt.NewObject()
	exports := r.rt.NewObject()
	if err := module.Set("exports", exports); err != nil {
		return err
	}
	if err := r.rt.Set("module", module); err != nil {
		return err
	}
	if err := r.rt.Set("exports", exports); err != nil {
		return err
	}
	if err := r.withInterrupt(func() error {
		_, err := r.rt.RunProgram(program)
		return err
	}); err != nil {
		return fmt.Errorf("run %s: %w", r.entry, err)
	}
	return nil
}
func (r *Runtime) Call(ctx context.Context, method string, args []any) (any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.exited {
		return nil, r.exitError
	}
	if ctx == nil {
		ctx = r.ctx
	}
	exports := r.rt.Get("module")
	if exports == nil || goja.IsUndefined(exports) {
		exports = r.rt.GlobalObject()
	}
	var fn goja.Callable
	if modObj, ok := exports.(*goja.Object); ok {
		expVal := modObj.Get("exports")
		if obj, ok := expVal.(*goja.Object); ok {
			if f, ok := goja.AssertFunction(obj.Get(method)); ok {
				fn = f
			}
		}
		if fn == nil {
			if f, ok := goja.AssertFunction(modObj.Get(method)); ok {
				fn = f
			}
		}
	}
	if fn == nil {
		return nil, fmt.Errorf("plugin does not export %q", method)
	}
	jsArgs := make([]goja.Value, 0, len(args))
	for _, a := range args {
		v, err := r.toJSValue(a)
		if err != nil {
			return nil, err
		}
		jsArgs = append(jsArgs, v)
	}
	promise, err := fn(goja.Undefined(), jsArgs...)
	if err != nil {
		return nil, err
	}
	return r.awaitPromise(ctx, promise)
}

// withInterrupt wraps a goja call with a fixed timeout interrupt guard.
// If the call does not complete within pluginCallTimeout, the goja runtime
// is interrupted and the call returns an error.
func (r *Runtime) withInterrupt(fn func() error) error {
	rt := r.rt
	timer := time.AfterFunc(pluginCallTimeout, func() {
		if rt != nil {
			rt.Interrupt("plugin call timed out")
		}
	})
	defer timer.Stop()

	execErr := fn()

	if rt := r.rt; rt != nil {
		rt.ClearInterrupt()
	}

	return execErr
}

func (r *Runtime) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.exited {
		return nil
	}
	r.exited = true
	r.rt = nil
	r.cancel()
	return nil
}
func (r *Runtime) awaitPromise(ctx context.Context, v goja.Value) (any, error) {
	if obj, ok := v.(*goja.Object); ok {
		if then, ok := goja.AssertFunction(obj.Get("then")); ok {
			resultCh := make(chan struct {
				val any
				err error
			}, 1)
			resolveCb := r.rt.ToValue(func(resolved goja.FunctionCall) goja.Value {
				val, _ := r.fromJSValue(resolved.Argument(0))
				resultCh <- struct {
					val any
					err error
				}{val, nil}
				return goja.Undefined()
			})
			rejectCb := r.rt.ToValue(func(call goja.FunctionCall) goja.Value {
				errVal, _ := r.fromJSValue(call.Argument(0))
				errMsg := fmt.Sprintf("plugin rejected: %v", errVal)
				resultCh <- struct {
					val any
					err error
				}{nil, errors.New(errMsg)}
				return goja.Undefined()
			})
			then(obj, resolveCb, rejectCb)
			select {
			case res := <-resultCh:
				return res.val, res.err
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	return r.fromJSValue(v)
}
func (r *Runtime) toJSValue(v any) (goja.Value, error) {
	switch x := v.(type) {
	case nil:
		return goja.Null(), nil
	case bool:
		return r.rt.ToValue(x), nil
	case string:
		return r.rt.ToValue(x), nil
	case int:
		return r.rt.ToValue(x), nil
	case int64:
		return r.rt.ToValue(x), nil
	case float64:
		return r.rt.ToValue(x), nil
	case []any:
		out := r.rt.NewArray()
		for i, item := range x {
			jv, err := r.toJSValue(item)
			if err != nil {
				return nil, err
			}
			if err := out.Set(strInt(i), jv); err != nil {
				return nil, err
			}
		}
		return out, nil
	case map[string]any:
		obj := r.rt.NewObject()
		for k, item := range x {
			jv, err := r.toJSValue(item)
			if err != nil {
				return nil, err
			}
			if err := obj.Set(k, jv); err != nil {
				return nil, err
			}
		}
		return obj, nil
	default:
		data, err := json.Marshal(x)
		if err != nil {
			return nil, err
		}
		var anyVal any
		if err := json.Unmarshal(data, &anyVal); err != nil {
			return nil, err
		}
		return r.toJSValue(anyVal)
	}
}
func (r *Runtime) fromJSValue(v goja.Value) (any, error) {
	if v == nil || goja.IsNull(v) || goja.IsUndefined(v) {
		return nil, nil
	}
	switch x := v.Export().(type) {
	case bool, string, int, int32, int64, float64, float32:
		return x, nil
	case []any:
		return x, nil
	case map[string]any:
		return x, nil
	default:
		raw := v.Export()
		data, err := json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		var out any
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
}
func strInt(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	buf := [20]byte{}
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = digits[i%10]
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
