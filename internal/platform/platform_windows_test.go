//go:build windows

package platform

import "testing"

func TestNew(t *testing.T) {
	pf := New()
	if pf == nil {
		t.Fatalf("New returned nil")
	}
	if _, ok := pf.(*winPlatform); !ok {
		t.Errorf("got %T, want *winPlatform", pf)
	}
}
