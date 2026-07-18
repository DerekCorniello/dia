package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Provenance records the remote a plugin was installed from, so
// `dia plugin update` knows what to re-clone. Plugins installed from a
// local path have none.
type Provenance struct {
	URL string `json:"url"`
	Ref string `json:"ref,omitempty"`
}

// writeProvenance stores the source alongside the installed plugin.
// A failure here is not fatal to the install: the plugin is already on
// disk and usable, it just will not be updatable in place.
func writeProvenance(dir string, p Provenance) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, provenanceFile), append(data, '\n'), 0o644)
}

// ReadProvenance returns the recorded source for an installed plugin.
// A missing file means the plugin came from a local path, which is not
// an error but leaves nothing to update from.
func ReadProvenance(dir string) (Provenance, bool, error) {
	data, err := os.ReadFile(filepath.Join(dir, provenanceFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Provenance{}, false, nil
		}
		return Provenance{}, false, fmt.Errorf("read plugin source: %w", err)
	}
	var p Provenance
	if err := json.Unmarshal(data, &p); err != nil {
		return Provenance{}, false, fmt.Errorf("parse plugin source: %w", err)
	}
	if p.URL == "" {
		return Provenance{}, false, nil
	}
	return p, true, nil
}
