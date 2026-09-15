package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func acquireDaemonLock(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create daemon lock directory: %w", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = f.WriteString(strconv.Itoa(os.Getpid()))
			_ = f.Sync()
			return f, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("acquire daemon lock: %w", err)
		}
		if owner, readErr := os.ReadFile(path); readErr == nil {
			if pid, parseErr := strconv.Atoi(string(owner)); parseErr == nil && pid > 0 && !daemonLockOwnerAlive(pid) {
				_ = os.Remove(path)
				continue
			}
		}
		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > time.Hour {
			_ = os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("daemon already running (lock %s)", path)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func releaseDaemonLock(path string, f *os.File) {
	_ = f.Close()
	_ = os.Remove(path)
}
