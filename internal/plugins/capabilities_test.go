package plugins

import (
	"reflect"
	"testing"
)

func TestKnownCapabilities(t *testing.T) {
	for _, c := range []string{
		CapWorkspacesRead, CapInstancesRead, CapDoctorRead, CapPathsRead, CapThemesRead,
		CapWorkspacesStart, CapInstancesStop, CapWorkspacesNew, CapThemesWrite,
	} {
		if !IsKnownCapability(c) {
			t.Errorf("%q should be known", c)
		}
	}
	if IsKnownCapability("not:a:cap") {
		t.Error("unknown cap should not be recognized")
	}
}
func TestMutatingClassification(t *testing.T) {
	for _, c := range []string{CapWorkspacesStart, CapInstancesStop, CapWorkspacesNew, CapThemesWrite} {
		if !IsMutatingCapability(c) {
			t.Errorf("%q should be mutating", c)
		}
	}
	for _, c := range []string{CapWorkspacesRead, CapInstancesRead, CapDoctorRead, CapPathsRead, CapThemesRead} {
		if IsMutatingCapability(c) {
			t.Errorf("%q should be read-only", c)
		}
	}
}
func TestMergeCapabilitiesDedupe(t *testing.T) {
	got := MergeCapabilities([]string{CapWorkspacesRead, CapInstancesRead}, []string{CapWorkspacesRead, CapWorkspacesStart})
	if len(got) != 3 {
		t.Errorf("expected 3 unique, got %d: %v", len(got), got)
	}
}
func TestMergeCapabilitiesIgnoresUnknown(t *testing.T) {
	got := MergeCapabilities([]string{CapWorkspacesRead}, []string{"bogus", CapInstancesStop})
	if len(got) != 2 {
		t.Errorf("expected 2 (unknown dropped), got %d: %v", len(got), got)
	}
}
func TestHasCapability(t *testing.T) {
	if !HasCapability([]string{CapWorkspacesRead, CapInstancesStop}, CapInstancesStop) {
		t.Error("expected true")
	}
	if HasCapability([]string{CapWorkspacesRead}, CapInstancesStop) {
		t.Error("expected false")
	}
}

func TestGrantCapabilities(t *testing.T) {
	tests := []struct {
		name      string
		requested []string
		granted   []string
		want      []string
	}{
		{
			name:      "intersection with stable order",
			requested: []string{CapWorkspacesRead, CapWorkspacesStart, CapDoctorRead},
			granted:   []string{CapWorkspacesStart, CapWorkspacesRead, "bogus"},
			want:      []string{CapWorkspacesStart, CapWorkspacesRead},
		},
		{
			name:      "drops unknown and out-of-set",
			requested: []string{CapWorkspacesRead},
			granted:   []string{CapWorkspacesStart, "made-up"},
			want:      []string{},
		},
		{
			name:      "dedupes when granted has repeats",
			requested: []string{CapWorkspacesRead, CapDoctorRead},
			granted:   []string{CapWorkspacesRead, CapWorkspacesRead, CapDoctorRead},
			want:      []string{CapWorkspacesRead, CapDoctorRead},
		},
		{
			name:      "empty",
			requested: nil,
			granted:   nil,
			want:      []string{},
		},
		{
			name:      "read defaults intersect",
			requested: []string{CapDoctorRead, CapWorkspacesStart},
			granted:   DefaultReadCapabilities(),
			want:      []string{CapDoctorRead},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GrantCapabilities(tc.requested, tc.granted)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
