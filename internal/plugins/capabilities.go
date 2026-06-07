package plugins

const (
	CapWorkspacesRead  = "workspaces:read"
	CapInstancesRead   = "instances:read"
	CapDoctorRead      = "doctor:read"
	CapPathsRead       = "paths:read"
	CapThemesRead      = "themes:read"
	CapWorkspacesStart = "workspaces:start"
	CapInstancesStop   = "instances:stop"
	CapWorkspacesNew   = "workspaces:create"
	CapThemesWrite     = "themes:write"
)

var defaultReadCaps = []string{
	CapWorkspacesRead,
	CapInstancesRead,
	CapDoctorRead,
	CapPathsRead,
	CapThemesRead,
}
var knownCapabilities = map[string]struct{}{
	CapWorkspacesRead:  {},
	CapInstancesRead:   {},
	CapDoctorRead:      {},
	CapPathsRead:       {},
	CapThemesRead:      {},
	CapWorkspacesStart: {},
	CapInstancesStop:   {},
	CapWorkspacesNew:   {},
	CapThemesWrite:     {},
}
var mutatingCapabilities = map[string]struct{}{
	CapWorkspacesStart: {},
	CapInstancesStop:   {},
	CapWorkspacesNew:   {},
	CapThemesWrite:     {},
}

func IsKnownCapability(c string) bool {
	_, ok := knownCapabilities[c]
	return ok
}
func IsMutatingCapability(c string) bool {
	_, ok := mutatingCapabilities[c]
	return ok
}
func IsReadCapability(c string) bool {
	_, ok := knownCapabilities[c]
	return ok && !IsMutatingCapability(c)
}
func DefaultReadCapabilities() []string {
	out := make([]string, len(defaultReadCaps))
	copy(out, defaultReadCaps)
	return out
}
func HasCapability(granted []string, want string) bool {
	for _, g := range granted {
		if g == want {
			return true
		}
	}
	return false
}
func MergeCapabilities(granted, requested []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(granted)+len(requested))
	for _, g := range granted {
		if _, ok := seen[g]; ok {
			continue
		}
		seen[g] = struct{}{}
		out = append(out, g)
	}
	for _, r := range requested {
		if _, ok := knownCapabilities[r]; !ok {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		out = append(out, r)
	}
	return out
}

// GrantCapabilities returns the subset of granted that is also
// declared in requested, deduplicated and order-stable. The intent
// is: requested is the manifest's cap set, granted is the
// user-approved (or default) cap set, and the returned slice is
// what the plugin actually gets. Unknown capabilities are dropped.
// Use this to apply a persisted grant list, or to derive a default
// set (e.g. all read caps) and intersect with the manifest.
func GrantCapabilities(requested, granted []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(granted))
	for _, g := range granted {
		if _, ok := seen[g]; ok {
			continue
		}
		if _, ok := knownCapabilities[g]; !ok {
			continue
		}
		if !inSlice(requested, g) {
			continue
		}
		seen[g] = struct{}{}
		out = append(out, g)
	}
	return out
}

func inSlice(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
