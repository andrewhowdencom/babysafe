package animation

import (
	"fmt"
	"strings"
	"sync/atomic"

	evdev "github.com/holoplot/go-evdev"
)

// Token is the display representation of one initial keyboard press.
type Token struct {
	Label string
}

// normalizer tracks the Shift state shared by all captured keyboards.
type normalizer struct {
	leftShift  atomic.Bool
	rightShift atomic.Bool
}

func newNormalizer() *normalizer { return &normalizer{} }

// normalize is safe for concurrent calls. Releases update modifier state, but
// only initial presses produce tokens.
func (n *normalizer) normalize(event *evdev.InputEvent) (Token, bool) {
	if event == nil || event.Type != evdev.EV_KEY {
		return Token{}, false
	}

	name := event.CodeName()
	if strings.HasPrefix(name, "BTN_") {
		return Token{}, false
	}

	switch event.Code {
	case evdev.KEY_LEFTSHIFT:
		n.leftShift.Store(event.Value != 0)
	case evdev.KEY_RIGHTSHIFT:
		n.rightShift.Store(event.Value != 0)
	}
	if event.Value != 1 {
		return Token{}, false
	}

	if label, ok := printableLabel(event.Code, n.shifted()); ok {
		return Token{Label: label}, true
	}
	return Token{Label: keyName(event)}, true
}

func (n *normalizer) shifted() bool {
	return n.leftShift.Load() || n.rightShift.Load()
}

func keyName(event *evdev.InputEvent) string {
	name := event.CodeName()
	if name == "unknown" {
		return fmt.Sprintf("KEY %d", event.Code)
	}
	name = strings.Split(name, "/")[0]
	name = strings.TrimPrefix(name, "KEY_")
	name = strings.ReplaceAll(name, "_", " ")
	return name
}

func printableLabel(code evdev.EvCode, shift bool) (string, bool) {
	if code == evdev.KEY_SPACE {
		return "SPACE", true
	}
	if r, ok := letterCodes[code]; ok {
		if shift {
			r -= 'a' - 'A'
		}
		return string(r), true
	}
	pair, ok := symbolCodes[code]
	if !ok {
		return "", false
	}
	if shift {
		return string(pair.shifted), true
	}
	return string(pair.plain), true
}

type runePair struct {
	plain   rune
	shifted rune
}

var symbolCodes = map[evdev.EvCode]runePair{
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

var letterCodes = map[evdev.EvCode]rune{
	evdev.KEY_A: 'a', evdev.KEY_B: 'b', evdev.KEY_C: 'c', evdev.KEY_D: 'd',
	evdev.KEY_E: 'e', evdev.KEY_F: 'f', evdev.KEY_G: 'g', evdev.KEY_H: 'h',
	evdev.KEY_I: 'i', evdev.KEY_J: 'j', evdev.KEY_K: 'k', evdev.KEY_L: 'l',
	evdev.KEY_M: 'm', evdev.KEY_N: 'n', evdev.KEY_O: 'o', evdev.KEY_P: 'p',
	evdev.KEY_Q: 'q', evdev.KEY_R: 'r', evdev.KEY_S: 's', evdev.KEY_T: 't',
	evdev.KEY_U: 'u', evdev.KEY_V: 'v', evdev.KEY_W: 'w', evdev.KEY_X: 'x',
	evdev.KEY_Y: 'y', evdev.KEY_Z: 'z',
}
