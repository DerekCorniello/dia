package runtime

import (
	"crypto/rand"
	"encoding/base32"
)

// newID returns a 12-character base32 identifier for a new Instance.
// 12 chars of base32 give ~60 bits of entropy, more than enough to
// disambiguate concurrent runs on a single host.
func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failures are unrecoverable: either the
		// kernel CSPRNG is broken or the system is shutting
		// down. In either case, panicking is correct.
		panic("runtime: crypto/rand failed: " + err.Error())
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
	return enc[:12]
}
