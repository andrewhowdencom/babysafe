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

func TestVirtualKeyboardCodesIncludeKeyboardIdentityAndExcludedKeys(t *testing.T) {
	codes := virtualKeyboardCodes([]evdev.EvCode{evdev.KEY_F13, evdev.KEY_F14, evdev.KEY_F13})
	seen := make(map[evdev.EvCode]bool)
	for _, code := range codes {
		if seen[code] {
			t.Fatalf("duplicate code %d", code)
		}
		seen[code] = true
	}
	for code := evdev.EvCode(evdev.KEY_ESC); code <= evdev.KEY_S; code++ {
		if !seen[code] {
			t.Errorf("keyboard identity lacks code %d", code)
		}
	}
	for _, code := range []evdev.EvCode{evdev.KEY_F13, evdev.KEY_F14} {
		if !seen[code] {
			t.Errorf("excluded key %d missing from virtual device", code)
		}
	}
	for _, code := range []evdev.EvCode{evdev.KEY_A, evdev.KEY_Z, evdev.KEY_ENTER} {
		if !seen[code] {
			t.Errorf("hotkey listener keyboard check lacks code %d", code)
		}
	}
}
