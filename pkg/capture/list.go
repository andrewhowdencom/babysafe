package capture

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	evdev "github.com/holoplot/go-evdev"
)

// InputDeviceGlob enumerates the kernel's input-event device nodes.
// Tests can override this to point at fixtures.
var InputDeviceGlob = "/dev/input/event*"

// Device describes a single input device as seen by babysafe. Path is
// always the resolved /dev/input/eventN file (not a by-id symlink) so
// downstream code never has to care about which alias the user typed.
type Device struct {
	Path  string
	Name  string
	Types []DeviceType
}

// DeviceType categorizes an input device. A device may belong to more
// than one category (e.g. a keyboard with a built-in touchpad).
type DeviceType string

const (
	DeviceTypeKeyboard DeviceType = "keyboard"
	DeviceTypeMouse    DeviceType = "mouse"
	DeviceTypeTouchpad DeviceType = "touchpad"
	DeviceTypeGamepad  DeviceType = "gamepad"
	DeviceTypeOther    DeviceType = "other"
)

// allDeviceTypes is the canonical, ordered list. Used for both
// validation (ParseDeviceType) and display ("all" types in --help).
var allDeviceTypes = []DeviceType{
	DeviceTypeKeyboard,
	DeviceTypeMouse,
	DeviceTypeTouchpad,
	DeviceTypeGamepad,
	DeviceTypeOther,
}

// ParseDeviceType maps a user-supplied string to a DeviceType. The
// match is case-insensitive; the empty string yields an error rather
// than silently defaulting, so a typo surfaces immediately.
func ParseDeviceType(s string) (DeviceType, error) {
	lc := strings.ToLower(s)
	for _, t := range allDeviceTypes {
		if string(t) == lc {
			return t, nil
		}
	}
	return "", fmt.Errorf("unknown device type %q (want one of %v)", s, allDeviceTypes)
}

// ListDevices enumerates every /dev/input/event* node and returns
// metadata about each. Devices that can't be opened (typically
// permission denied) are skipped with a warning — listing should never
// hard-fail, because the user may legitimately lack access to a few
// nodes while still being able to grab others.
func ListDevices(ctx context.Context, logger *slog.Logger) ([]*Device, error) {
	if logger == nil {
		logger = slog.Default()
	}

	paths, err := filepath.Glob(InputDeviceGlob)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", InputDeviceGlob, err)
	}

	var out []*Device
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		dev, err := evdev.Open(p)
		if err != nil {
			logger.Warn("could not open input device", "path", p, "err", err)
			continue
		}

		name, _ := dev.Name()
		types := DetectTypes(dev)

		// Close immediately — we don't need to hold the fd for
		// listing, and a held fd would block the actual grab.
		_ = dev.Close()

		out = append(out, &Device{
			Path:  p,
			Name:  name,
			Types: types,
		})
	}

	return out, nil
}

// DetectTypes inspects a device's capabilities and returns the
// categories it belongs to. A device with no recognizable capability
// set is reported as DeviceTypeOther so the user can still grab it
// explicitly.
func DetectTypes(dev *evdev.InputDevice) []DeviceType {
	var types []DeviceType

	keys := dev.CapableEvents(evdev.EV_KEY)
	rels := dev.CapableEvents(evdev.EV_REL)
	abs := dev.CapableEvents(evdev.EV_ABS)

	if isKeyboard(keys) {
		types = append(types, DeviceTypeKeyboard)
	}
	if isMouse(rels) {
		types = append(types, DeviceTypeMouse)
	}
	if isTouchpad(abs) {
		types = append(types, DeviceTypeTouchpad)
	}
	if isGamepad(keys) {
		types = append(types, DeviceTypeGamepad)
	}
	if len(types) == 0 {
		types = append(types, DeviceTypeOther)
	}
	return types
}

// isKeyboard: has at least a couple of the canonical letter keys.
// This avoids classifying a media-key remote (which only has EV_KEY
// for consumer transport controls) as a keyboard.
func isKeyboard(keys []evdev.EvCode) bool {
	hits := 0
	for _, k := range keys {
		if k == evdev.KEY_A || k == evdev.KEY_Z ||
			k == evdev.KEY_1 || k == evdev.KEY_SPACE ||
			k == evdev.KEY_ENTER {
			hits++
			if hits >= 2 {
				return true
			}
		}
	}
	return false
}

// isMouse: has relative axes (REL_X / REL_Y). Touchpads also report
// REL_X in some configurations; we accept that and let the user
// disambiguate via --match if they care.
func isMouse(rels []evdev.EvCode) bool {
	hasX, hasY := false, false
	for _, r := range rels {
		if r == evdev.REL_X {
			hasX = true
		}
		if r == evdev.REL_Y {
			hasY = true
		}
	}
	return hasX && hasY
}

// isTouchpad: has ABS_MT_POSITION_X (multi-touch) or ABS_X (single-touch).
func isTouchpad(abs []evdev.EvCode) bool {
	for _, a := range abs {
		if a == evdev.ABS_MT_POSITION_X || a == evdev.ABS_X {
			return true
		}
	}
	return false
}

// isGamepad: has any of the BTN_GAMEPAD / BTN_SOUTH / BTN_TRIGGER
// style buttons.
func isGamepad(keys []evdev.EvCode) bool {
	for _, k := range keys {
		if k == evdev.BTN_GAMEPAD || k == evdev.BTN_SOUTH ||
			k == evdev.BTN_TRIGGER || k == evdev.BTN_START {
			return true
		}
	}
	return false
}
