package cli

import (
	"sync"

	evdev "github.com/holoplot/go-evdev"
)

// echoDecoder translates an evdev input event into the string that the
// user would see if the keyboard were not grabbed: printable keys come
// back as the character they type (taking Shift into account), and
// special keys come back as bracketed names like "[ENTER]".
//
// It tracks modifier state across calls and is goroutine-safe so it
// can be shared across the per-device drain goroutines. One decoder
// is sufficient — its state is the union of what the user is doing on
// the keyboard(s) the grab holds.
type echoDecoder struct {
	mu sync.Mutex

	// Modifiers tracked explicitly. Shift is enough to disambiguate
	// case and the symbol row; the others are kept so future
	// extensions (e.g. emitting "^C" for Ctrl+C) have the data on
	// hand without revisiting this code.
	shift bool
	ctrl  bool
	alt   bool
	meta  bool
}

// feed advances the decoder's state and returns the string to print
// for this event, or "" if the event produces no output (release
// events and modifier presses).
//
// As with modifierState.update, autorepeat (ev.Value == 2) is
// treated as "still held" so holding Shift + A prints 'A', not 'a'.
// Releases (ev.Value == 0) are treated as "up".
func (d *echoDecoder) feed(ev *evdev.InputEvent) string {
	if ev.Type != evdev.EV_KEY {
		return ""
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	isDown := ev.Value != 0
	switch ev.Code {
	case evdev.KEY_LEFTSHIFT, evdev.KEY_RIGHTSHIFT:
		d.shift = isDown
		return ""
	case evdev.KEY_LEFTCTRL, evdev.KEY_RIGHTCTRL:
		d.ctrl = isDown
		return ""
	case evdev.KEY_LEFTALT, evdev.KEY_RIGHTALT:
		d.alt = isDown
		return ""
	case evdev.KEY_LEFTMETA, evdev.KEY_RIGHTMETA:
		d.meta = isDown
		return ""
	}

	// Releases produce nothing. Presses (1) and autorepeats (2)
	// both produce output — holding K prints "KKKK...", which is
	// what the user would actually see at the keyboard.
	if ev.Value == 0 {
		return ""
	}

	if r, ok := decodePrintable(ev.Code, d.shift); ok {
		return string(r)
	}

	if name, ok := decodeSpecial(ev.Code); ok {
		return "[" + name + "]"
	}

	return ""
}

// decodePrintable returns the rune a US-layout key produces for the
// given shift state, or (0, false) for keys it does not know.
func decodePrintable(code evdev.EvCode, shift bool) (rune, bool) {
	// Letters: the evdev keycodes are not contiguous or in any
	// predictable order (KEY_A=30, KEY_B=48, KEY_C=46, …), so a
	// table is the only sane mapping.
	if r, ok := letterCodes[code]; ok {
		if shift {
			return r - 'a' + 'A', true
		}
		return r, true
	}

	// Symbol row + punctuation — explicit (unshifted, shifted) pairs.
	type pair struct{ plain, shifted rune }
	pairs := map[evdev.EvCode]pair{
		evdev.KEY_1:          {'1', '!'},
		evdev.KEY_2:          {'2', '@'},
		evdev.KEY_3:          {'3', '#'},
		evdev.KEY_4:          {'4', '$'},
		evdev.KEY_5:          {'5', '%'},
		evdev.KEY_6:          {'6', '^'},
		evdev.KEY_7:          {'7', '&'},
		evdev.KEY_8:          {'8', '*'},
		evdev.KEY_9:          {'9', '('},
		evdev.KEY_0:          {'0', ')'},
		evdev.KEY_MINUS:      {'-', '_'},
		evdev.KEY_EQUAL:      {'=', '+'},
		evdev.KEY_LEFTBRACE:  {'[', '{'},
		evdev.KEY_RIGHTBRACE: {']', '}'},
		evdev.KEY_SEMICOLON:  {';', ':'},
		evdev.KEY_APOSTROPHE: {'\'', '"'},
		evdev.KEY_GRAVE:      {'`', '~'},
		evdev.KEY_BACKSLASH:  {'\\', '|'},
		evdev.KEY_COMMA:      {',', '<'},
		evdev.KEY_DOT:        {'.', '>'},
		evdev.KEY_SLASH:      {'/', '?'},
	}

	if p, ok := pairs[code]; ok {
		if shift {
			return p.shifted, true
		}
		return p.plain, true
	}

	if code == evdev.KEY_SPACE {
		return ' ', true
	}

	return 0, false
}

// letterCodes maps the US-letter evdev keycodes to their lowercase
// rune. The evdev keycodes are not contiguous (KEY_A=30, KEY_B=48,
// KEY_C=46, …), so a table is the only correct mapping.
var letterCodes = map[evdev.EvCode]rune{
	evdev.KEY_A: 'a',
	evdev.KEY_B: 'b',
	evdev.KEY_C: 'c',
	evdev.KEY_D: 'd',
	evdev.KEY_E: 'e',
	evdev.KEY_F: 'f',
	evdev.KEY_G: 'g',
	evdev.KEY_H: 'h',
	evdev.KEY_I: 'i',
	evdev.KEY_J: 'j',
	evdev.KEY_K: 'k',
	evdev.KEY_L: 'l',
	evdev.KEY_M: 'm',
	evdev.KEY_N: 'n',
	evdev.KEY_O: 'o',
	evdev.KEY_P: 'p',
	evdev.KEY_Q: 'q',
	evdev.KEY_R: 'r',
	evdev.KEY_S: 's',
	evdev.KEY_T: 't',
	evdev.KEY_U: 'u',
	evdev.KEY_V: 'v',
	evdev.KEY_W: 'w',
	evdev.KEY_X: 'x',
	evdev.KEY_Y: 'y',
	evdev.KEY_Z: 'z',
}

// decodeSpecial returns a printable name for non-character keys
// (Enter, Tab, the navigation cluster, function keys), or
// ("", false) for keys that should produce no output.
func decodeSpecial(code evdev.EvCode) (string, bool) {
	switch code {
	case evdev.KEY_ENTER:
		return "ENTER", true
	case evdev.KEY_TAB:
		return "TAB", true
	case evdev.KEY_BACKSPACE:
		return "BACKSPACE", true
	case evdev.KEY_ESC:
		return "ESC", true
	case evdev.KEY_DELETE:
		return "DELETE", true
	case evdev.KEY_UP:
		return "UP", true
	case evdev.KEY_DOWN:
		return "DOWN", true
	case evdev.KEY_LEFT:
		return "LEFT", true
	case evdev.KEY_RIGHT:
		return "RIGHT", true
	case evdev.KEY_HOME:
		return "HOME", true
	case evdev.KEY_END:
		return "END", true
	case evdev.KEY_PAGEUP:
		return "PAGEUP", true
	case evdev.KEY_PAGEDOWN:
		return "PAGEDOWN", true
	case evdev.KEY_INSERT:
		return "INSERT", true
	case evdev.KEY_F1:
		return "F1", true
	case evdev.KEY_F2:
		return "F2", true
	case evdev.KEY_F3:
		return "F3", true
	case evdev.KEY_F4:
		return "F4", true
	case evdev.KEY_F5:
		return "F5", true
	case evdev.KEY_F6:
		return "F6", true
	case evdev.KEY_F7:
		return "F7", true
	case evdev.KEY_F8:
		return "F8", true
	case evdev.KEY_F9:
		return "F9", true
	case evdev.KEY_F10:
		return "F10", true
	case evdev.KEY_F11:
		return "F11", true
	case evdev.KEY_F12:
		return "F12", true
	}
	return "", false
}
