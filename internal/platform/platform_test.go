//go:build linux || darwin

package platform

import (
	"testing"
)

func TestNew(t *testing.T) {
	pf := New()
	if pf == nil {
		t.Fatalf("New returned nil")
	}
	if _, ok := pf.(*unixPlatform); !ok {
		t.Errorf("got %T, want *unixPlatform", pf)
	}
}

