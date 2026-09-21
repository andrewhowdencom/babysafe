package animation

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestNewANSITerminalRejectsNonTTY(t *testing.T) {
	_, err := newANSITerminal(&bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("newANSITerminal() error = %v", err)
	}
}

func TestANSITerminalRestoreIsIdempotent(t *testing.T) {
	var output bytes.Buffer
	terminal := &ansiTerminal{writer: &output, entered: true}
	if err := terminal.restore(); err != nil {
		t.Fatal(err)
	}
	if err := terminal.restore(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(output.String(), restoreSequence); got != 1 {
		t.Fatalf("restore sequence count = %d, want 1", got)
	}
}

func TestNewANSITerminalRejectsDumbTermBeforeWriting(t *testing.T) {
	t.Setenv("TERM", "dumb")
	file, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	_, err = newANSITerminal(file)
	if err == nil || !strings.Contains(err.Error(), "color-capable") {
		t.Fatalf("newANSITerminal() error = %v", err)
	}
	info, statErr := file.Stat()
	if statErr != nil {
		t.Fatal(statErr)
	}
	if info.Size() != 0 {
		t.Fatalf("preflight wrote %d bytes", info.Size())
	}
}
