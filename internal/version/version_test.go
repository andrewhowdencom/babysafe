package version

import "testing"

func TestString(t *testing.T) {
	const want = "v9.9.9"
	Version = want
	t.Cleanup(func() { Version = "v0.0.0-dev" })

	if got := String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
