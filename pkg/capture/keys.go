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
