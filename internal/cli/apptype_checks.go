package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DerekCorniello/dia/internal/diag"
	"github.com/DerekCorniello/dia/internal/plugins"
)

// appTypeChecks reports the app types plugins provide, and warns about
// any type two plugins both claimed. A conflict is silent otherwise:
// the workspace still starts, it just launches the other plugin's idea
// of that type, which is exactly the kind of thing doctor exists for.
func appTypeChecks(pmgr *plugins.Manager) []diag.Check {
	if pmgr == nil {
		return nil
	}
	var checks []diag.Check

	claimed := pmgr.AppTypes()
	if len(claimed) > 0 {
		parts := make([]string, 0, len(claimed))
		for typeName, id := range claimed {
			parts = append(parts, fmt.Sprintf("%s (%s)", typeName, id))
		}
		sort.Strings(parts)
		checks = append(checks, diag.Check{
			Name:   "plugin app types",
			Status: "ok",
			Detail: strings.Join(parts, ", "),
		})
	}

	for _, c := range pmgr.AppTypeConflicts() {
		checks = append(checks, diag.Check{
			Name:   "app type conflict",
			Status: "warn",
			Detail: fmt.Sprintf("%q claimed by %s and %s; using %s", c.Type, c.Winner, c.Loser, c.Winner),
		})
	}
	return checks
}
