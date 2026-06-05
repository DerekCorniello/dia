package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolvePath expands a leading "~" to the user's home directory and
// any "$VAR" / "${VAR}" tokens to environment variable values. It does
// not touch the filesystem beyond reading the env; the caller is
// responsible for confirming the path exists.
func resolvePath(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve ~: %w", err)
		}
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return os.ExpandEnv(p), nil
}
