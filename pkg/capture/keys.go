package capture

import (
	"fmt"
	"strings"

	evdev "github.com/holoplot/go-evdev"
)

// ParseKeys resolves Linux KEY_* names for keys that should reach the host.
func ParseKeys(names []string) (map[evdev.EvCode]bool, error) {
	keys := make(map[evdev.EvCode]bool, len(names))
	for _, name := range names {
		name = strings.ToUpper(strings.TrimSpace(name))
		if !strings.HasPrefix(name, "KEY_") {
			name = "KEY_" + name
		}
		found := false
		for code, known := range evdev.KEYNames {
			if known == name {
				keys[code] = true
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown key %q (use a Linux KEY_* name, e.g. KEY_F13)", name)
		}
	}
	return keys, nil
}

// virtualKeyboardCodes makes the passthrough device recognizable as a
// keyboard by udev. Only the excluded keys are ever emitted on it.
func virtualKeyboardCodes(excluded []evdev.EvCode) []evdev.EvCode {
	codes := make([]evdev.EvCode, 0, int(evdev.KEY_S-evdev.KEY_ESC+2)+len(excluded))
	seen := make(map[evdev.EvCode]bool, cap(codes))
	// udev identifies a full keyboard by the first 31 key capabilities.
	for code := evdev.EvCode(evdev.KEY_ESC); code <= evdev.KEY_S; code++ {
		codes = append(codes, code)
		seen[code] = true
	}
	// Some evdev listeners require A, Z, and Enter before opening a
	// device as a keyboard. Z is outside the udev identity range above.
	codes = append(codes, evdev.KEY_Z)
	seen[evdev.KEY_Z] = true
	for _, code := range excluded {
		if !seen[code] {
			codes = append(codes, code)
			seen[code] = true
		}
	}
	return codes
}
