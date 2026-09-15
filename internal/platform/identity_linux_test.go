//go:build linux

package platform

import (
	"os"
	"testing"
)

func TestProcessIdentityCurrentProcess(t *testing.T) {
	p := unixPlatform{}
	identity, err := p.ProcessIdentity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if identity == "" {
		t.Fatal("ProcessIdentity returned an empty token")
	}
	if second, err := p.ProcessIdentity(os.Getpid()); err != nil || second != identity {
		t.Fatalf("identity changed for same process: %q then %q (%v)", identity, second, err)
	}
}
