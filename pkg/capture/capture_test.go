package capture

import (
	"path/filepath"
	"reflect"
	"testing"
)

// TestResolveDevices exercises the glob-expansion + exclude logic
// without touching the kernel — those paths live in integration tests.
func TestResolveDevices(t *testing.T) {
	tests := []struct {
		name     string
		globs    []string
		excludes []string
		wantErr  bool
		// wantMatches is checked as a set against the result. Paths
		// that don't exist on the test host are filtered out.
		wantMatches []string
	}{
		{
			name:        "glob expansion returns at least one match when present",
			globs:       []string{filepath.Join(t.TempDir(), "event*")},
			wantMatches: nil, // TempDir is empty, so the result should be empty -> error.
			wantErr:     true,
		},
		{
			name:    "invalid glob",
			globs:   []string{"["},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveDevices(tt.globs, tt.excludes)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveDevices() err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.wantMatches) {
				t.Errorf("resolveDevices() = %v, want %v", got, tt.wantMatches)
			}
		})
	}
}

// TestResolveDevicesWithFixtures creates a temporary directory tree
// that mimics /dev/input and verifies the expansion/exclude logic.
func TestResolveDevicesWithFixtures(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"event0", "event1", "event2", "mice"} {
		if err := writeEmptyFile(filepath.Join(tmp, name)); err != nil {
			t.Fatalf("setup %s: %v", name, err)
		}
	}

	got, err := resolveDevices([]string{filepath.Join(tmp, "event*")}, nil)
	if err != nil {
		t.Fatalf("resolveDevices: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("len(got) = %d, want 3 (got %v)", len(got), got)
	}

	got, err = resolveDevices([]string{filepath.Join(tmp, "event*")}, []string{"event1"})
	if err != nil {
		t.Fatalf("resolveDevices with exclude: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2 (got %v)", len(got), got)
	}
	for _, p := range got {
		if filepath.Base(p) == "event1" {
			t.Errorf("event1 should have been excluded, got %v", got)
		}
	}
}

func writeEmptyFile(path string) error {
	return writeFile(path, 0o600)
}
