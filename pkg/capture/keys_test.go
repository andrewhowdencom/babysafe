package capture

import (
	"testing"

	evdev "github.com/holoplot/go-evdev"
)

func TestParseKeys(t *testing.T) {
	keys, err := ParseKeys([]string{"key_f13", "F14", " KEY_LEFTCTRL "})
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []evdev.EvCode{evdev.KEY_F13, evdev.KEY_F14, evdev.KEY_LEFTCTRL} {
		if !keys[code] {
			t.Errorf("missing code %d", code)
		}
	}
	if _, err := ParseKeys([]string{"key_not_real"}); err == nil {
		t.Fatal("unknown key was accepted")
	}
}

func TestShouldForwardOnlySelectedKeyEvents(t *testing.T) {
	keys := map[evdev.EvCode]bool{evdev.KEY_F13: true}
	for _, value := range []int32{0, 1, 2} {
		if !shouldForward(&evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.KEY_F13, Value: value}, keys) {
			t.Errorf("F13 event with value %d was not forwarded", value)
		}
	}
	if shouldForward(&evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.KEY_F14, Value: 1}, keys) ||
		shouldForward(&evdev.InputEvent{Type: evdev.EV_MSC, Code: evdev.KEY_F13}, keys) {
		t.Fatal("unselected event was forwarded")
	}
}
