package animation

import (
	"fmt"
	"sync"
	"testing"

	evdev "github.com/holoplot/go-evdev"
)

func TestNormalizer(t *testing.T) {
	tests := []struct {
		name  string
		event evdev.InputEvent
		want  string
		ok    bool
	}{
		{name: "lowercase", event: keyEvent(evdev.KEY_A, 1), want: "a", ok: true},
		{name: "space", event: keyEvent(evdev.KEY_SPACE, 1), want: "SPACE", ok: true},
		{name: "punctuation", event: keyEvent(evdev.KEY_SLASH, 1), want: "/", ok: true},
		{name: "modifier", event: keyEvent(evdev.KEY_LEFTCTRL, 1), want: "LEFTCTRL", ok: true},
		{name: "arrow", event: keyEvent(evdev.KEY_UP, 1), want: "UP", ok: true},
		{name: "function", event: keyEvent(evdev.KEY_F12, 1), want: "F12", ok: true},
		{name: "media", event: keyEvent(evdev.KEY_VOLUMEUP, 1), want: "VOLUMEUP", ok: true},
		{name: "unknown", event: keyEvent(evdev.EvCode(0xffff), 1), want: "KEY 65535", ok: true},
		{name: "release", event: keyEvent(evdev.KEY_A, 0)},
		{name: "autorepeat", event: keyEvent(evdev.KEY_A, 2)},
		{name: "non-key", event: evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_X, Value: 1}},
		{name: "mouse button", event: keyEvent(evdev.BTN_LEFT, 1)},
		{name: "gamepad button", event: keyEvent(evdev.BTN_GAMEPAD, 1)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token, ok := newNormalizer().normalize(&test.event)
			if ok != test.ok || token.Label != test.want {
				t.Fatalf("normalize() = (%q, %v), want (%q, %v)", token.Label, ok, test.want, test.ok)
			}
		})
	}
}

func TestNormalizerTracksBothShiftKeys(t *testing.T) {
	n := newNormalizer()
	_, _ = n.normalize(eventPtr(keyEvent(evdev.KEY_LEFTSHIFT, 1)))
	_, _ = n.normalize(eventPtr(keyEvent(evdev.KEY_RIGHTSHIFT, 1)))
	_, _ = n.normalize(eventPtr(keyEvent(evdev.KEY_LEFTSHIFT, 0)))

	token, ok := n.normalize(eventPtr(keyEvent(evdev.KEY_1, 1)))
	if !ok || token.Label != "!" {
		t.Fatalf("shifted token = (%q, %v), want (!, true)", token.Label, ok)
	}
}

func TestNormalizerConcurrent(t *testing.T) {
	n := newNormalizer()
	var wait sync.WaitGroup
	values := [...]int32{0, 1, 2}
	for i := 0; i < 100; i++ {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			event := keyEvent(evdev.KEY_A, values[i%len(values)])
			_, _ = n.normalize(&event)
		}(i)
	}
	wait.Wait()
}

func keyEvent(code evdev.EvCode, value int32) evdev.InputEvent {
	return evdev.InputEvent{Type: evdev.EV_KEY, Code: code, Value: value}
}

func eventPtr(event evdev.InputEvent) *evdev.InputEvent { return &event }

func TestKeyNameRemovesTechnicalPrefix(t *testing.T) {
	event := keyEvent(evdev.KEY_BRIGHTNESSUP, 1)
	if got := keyName(&event); got != "BRIGHTNESSUP" {
		t.Errorf("keyName() = %q", got)
	}
	if got := fmt.Sprint(event.Code); got == "" {
		t.Fatal("unexpected empty code")
	}
}
