package state

import (
	"encoding/json"
	"testing"
)

func FuzzMigrateRecentNeverPanics(f *testing.F) {
	f.Add([]byte(`"alpha"`))
	f.Add([]byte(`{"name":"alpha","count":1}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, input []byte) {
		var raw []json.RawMessage
		if err := json.Unmarshal(input, &raw); err != nil {
			return
		}
		_ = migrateRecent(raw)
	})
}
