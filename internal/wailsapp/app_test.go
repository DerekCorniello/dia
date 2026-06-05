package wailsapp

import (
	"context"
	"testing"
)

func TestNewReturnsApp(t *testing.T) {
	a := New()
	if a == nil {
		t.Fatal("New returned nil")
	}
	if a.ctx != nil {
		t.Fatal("newly constructed App should have nil context")
	}
}

func TestStartupStoresContext(t *testing.T) {
	a := New()
	ctx := context.Background()
	a.Startup(ctx)
	if a.ctx != ctx {
		t.Fatal("Startup did not store context")
	}
}
