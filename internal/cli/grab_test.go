package cli

import "testing"

func TestHoldingMessageShowsBreakCombination(t *testing.T) {
	const want = "babysafe: holding 2 device(s); press Ctrl+Alt+Esc to release.\n"
	if got := holdingMessage(2); got != want {
		t.Errorf("holdingMessage() = %q, want %q", got, want)
	}
}
