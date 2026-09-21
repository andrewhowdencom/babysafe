package cli

import (
	"io"
	"strings"
	"testing"

	evdev "github.com/holoplot/go-evdev"
)

func TestRenderAnimationFrame(t *testing.T) {
	tests := []struct {
		name      string
		animation keyAnimation
		frame     int
		want      string
	}{
		{name: "idle prompt", frame: -1, want: "PRESS ANY KEY"},
		{name: "pressed key", animation: keyAnimation{label: "A", seed: 30}, frame: 3, want: "A"},
		{name: "special key", animation: keyAnimation{label: "ENTER", seed: 28}, frame: 5, want: "ENTER"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderAnimationFrame(tt.animation, tt.frame)
			if !strings.Contains(got, tt.want) {
				t.Errorf("renderAnimationFrame() did not contain %q", tt.want)
			}
			if !strings.Contains(got, "Ctrl+Alt+Esc to finish") {
				t.Error("renderAnimationFrame() omitted the release hint")
			}
		})
	}
}

func TestModifierLabel(t *testing.T) {
	tests := []struct {
		code evdev.EvCode
		want string
	}{
		{code: evdev.KEY_LEFTSHIFT, want: "SHIFT"},
		{code: evdev.KEY_RIGHTCTRL, want: "CTRL"},
		{code: evdev.KEY_LEFTALT, want: "ALT"},
		{code: evdev.KEY_RIGHTMETA, want: "SUPER"},
		{code: evdev.KEY_A, want: ""},
	}

	for _, tt := range tests {
		if got := modifierLabel(tt.code); got != tt.want {
			t.Errorf("modifierLabel(%v) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestKeyAnimatorHandlesPressesOnly(t *testing.T) {
	animator := newKeyAnimator(io.Discard)

	animator.handle(&evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.KEY_A, Value: 0})
	if got := len(animator.events); got != 0 {
		t.Fatalf("release queued %d animations, want 0", got)
	}

	animator.handle(&evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTSHIFT, Value: 1})
	animation := <-animator.events
	if animation.label != "SHIFT" {
		t.Errorf("modifier animation label = %q, want SHIFT", animation.label)
	}

	animator.handle(&evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.KEY_A, Value: 1})
	animation = <-animator.events
	if animation.label != "A" {
		t.Errorf("shifted key animation label = %q, want A", animation.label)
	}
}

func TestCenteredText(t *testing.T) {
	if got, want := centeredText("★", 5), "  ★  "; got != want {
		t.Errorf("centeredText() = %q, want %q", got, want)
	}
}
