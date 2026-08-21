package capture

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	evdev "github.com/holoplot/go-evdev"
)

// TestListDevicesWithFixtures creates a synthetic /dev/input layout
// in a temp dir and re-points the package's glob at it. This avoids
// the need for a real device on the test host.
func TestListDevicesWithFixtures(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"event0", "event1", "event2"} {
		if err := writeFile(filepath.Join(tmp, name), 0o600); err != nil {
			t.Fatalf("setup %s: %v", name, err)
		}
	}

	// Override the glob, then restore.
	orig := InputDeviceGlob
	InputDeviceGlob = filepath.Join(tmp, "event*")
	t.Cleanup(func() { InputDeviceGlob = orig })

	devs, err := ListDevices(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}

	// Each fixture is an empty file, so opening it will fail (it's not
	// a real evdev device). ListDevices should skip them and return
	// an empty slice — the test is then really about the globbing
	// code path not panicking.
	if len(devs) != 0 {
		t.Errorf("ListDevices returned %d devices, want 0 (fixtures are not real evdev nodes)", len(devs))
	}
}

func TestParseDeviceType(t *testing.T) {
	tests := []struct {
		in      string
		want    DeviceType
		wantErr bool
	}{
		{"keyboard", DeviceTypeKeyboard, false},
		{"Keyboard", DeviceTypeKeyboard, false},
		{"KEYBOARD", DeviceTypeKeyboard, false},
		{"mouse", DeviceTypeMouse, false},
		{"touchpad", DeviceTypeTouchpad, false},
		{"gamepad", DeviceTypeGamepad, false},
		{"other", DeviceTypeOther, false},
		{"", "", true},
		{"glasses", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseDeviceType(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseDeviceType(%q) err = %v, wantErr = %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseDeviceType(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseMatchers(t *testing.T) {
	tests := []struct {
		name    string
		exprs   []string
		wantErr bool
		// wantCount is the number of matchers that should be produced
		// when wantErr is false.
		wantCount int
	}{
		{"empty", nil, false, 0},
		{"single valid", []string{"type=keyboard"}, false, 1},
		{"multiple valid", []string{"path=/dev/input/event*", "name=Logitech", "type=keyboard"}, false, 3},
		{"missing equals", []string{"type keyboard"}, true, 0},
		{"empty value", []string{"type="}, true, 0},
		{"unknown key", []string{"model=Logitech"}, true, 0},
		{"unknown type", []string{"type=glasses"}, false, 1}, // Produces a no-match matcher rather than error.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMatchers(tt.exprs)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseMatchers(%v) err = %v, wantErr = %v", tt.exprs, err, tt.wantErr)
			}
			if !tt.wantErr && len(got) != tt.wantCount {
				t.Errorf("ParseMatchers(%v) returned %d matchers, want %d", tt.exprs, len(got), tt.wantCount)
			}
		})
	}
}

func TestMatchersApply(t *testing.T) {
	dev := &Device{
		Path:  "/dev/input/event1",
		Name:  "Logitech G512",
		Types: []DeviceType{DeviceTypeKeyboard},
	}

	tests := []struct {
		name string
		m    Matcher
		want bool
	}{
		{"name substring", nameMatcher("logitech"), true},
		{"name case insensitive", nameMatcher("LOGI"), true},
		{"name miss", nameMatcher("razer"), false},
		{"type hit", typeMatcher(DeviceTypeKeyboard), true},
		{"type miss", typeMatcher(DeviceTypeMouse), false},
		{"type multi", typeMatcher(DeviceTypeMouse, DeviceTypeKeyboard), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m(dev); got != tt.want {
				t.Errorf("matcher = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchAllAndMatchAny(t *testing.T) {
	dev := &Device{Path: "/x", Name: "Logitech", Types: []DeviceType{DeviceTypeKeyboard}}
	m := nameMatcher("logitech")

	if !matchAll(dev, nil) {
		t.Errorf("matchAll with no matchers should be true")
	}
	if !matchAll(dev, []Matcher{m}) {
		t.Errorf("matchAll with one passing matcher should be true")
	}
	if matchAll(dev, []Matcher{m, nameMatcher("razer")}) {
		t.Errorf("matchAll with one failing matcher should be false")
	}

	if !matchAny(dev, []Matcher{m, nameMatcher("razer")}) {
		t.Errorf("matchAny with one passing matcher should be true")
	}
	if matchAny(dev, []Matcher{nameMatcher("razer")}) {
		t.Errorf("matchAny with no passing matchers should be false")
	}
}

func TestMatchExprError(t *testing.T) {
	err := &MatchExprError{Expr: "junk", Reason: "expected key=value"}
	if got := err.Error(); got == "" || got == "internal: invalid match expression" {
		t.Errorf("Error() = %q, want it to mention the expression and reason", got)
	}
}

// TestHelpersMakeSureUsefulTypesStillExist exists to catch accidental
// removal of the public types the CLI depends on.
func TestPublicTypesStable(t *testing.T) {
	types := []DeviceType{
		DeviceTypeKeyboard, DeviceTypeMouse,
		DeviceTypeTouchpad, DeviceTypeGamepad, DeviceTypeOther,
	}
	want := []string{"keyboard", "mouse", "touchpad", "gamepad", "other"}
	got := make([]string, len(types))
	for i, t := range types {
		got[i] = string(t)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DeviceType string values changed: got %v, want %v", got, want)
	}
}

// TestModifierState exercises the modifier-tracking logic that the
// drain loop uses to detect the Ctrl+Alt+Esc break-out combo. The
// table cases cover the matching combo, both half-combos, the bare
// trigger key, the right-hand variants, and the release path.
func TestModifierState(t *testing.T) {
	tests := []struct {
		name      string
		events    []*evdev.InputEvent
		wantBreak bool
	}{
		{
			name: "ctrl+alt+esc triggers",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTCTRL, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTALT, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_ESC, Value: 1},
			},
			wantBreak: true,
		},
		{
			name: "right-hand modifiers also trigger",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_RIGHTCTRL, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_RIGHTALT, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_ESC, Value: 1},
			},
			wantBreak: true,
		},
		{
			name: "ctrl+esc without alt does not trigger",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTCTRL, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_ESC, Value: 1},
			},
			wantBreak: false,
		},
		{
			name: "alt+esc without ctrl does not trigger",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTALT, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_ESC, Value: 1},
			},
			wantBreak: false,
		},
		{
			name: "esc alone does not trigger",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_ESC, Value: 1},
			},
			wantBreak: false,
		},
		{
			name: "release of esc does not retrigger",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTCTRL, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTALT, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_ESC, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_ESC, Value: 0},
			},
			wantBreak: true, // first press is the trigger; release must not retrigger
		},
		{
			name: "non-key events between modifiers are ignored",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTCTRL, Value: 1},
				{Type: evdev.EV_REL, Code: 0, Value: 1}, // mouse-move-shaped noise
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTALT, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_ESC, Value: 1},
			},
			wantBreak: true,
		},
		{
			// The kernel sends autorepeat (Value=2) for held keys.
			// A modifier that has been "released" by an autorepeat
			// event wouldn't be recognized as held, so the combo
			// would never fire while a key is being held.
			name: "ctrl held through autorepeat still triggers with alt+esc",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTCTRL, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTCTRL, Value: 2}, // autorepeat
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTCTRL, Value: 2}, // autorepeat
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTALT, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_ESC, Value: 1},
			},
			wantBreak: true,
		},
		{
			// Conversely, autorepeat must not spuriously trigger
			// the combo when only one modifier is held.
			name: "ctrl autorepeat alone does not trigger",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTCTRL, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTCTRL, Value: 2},
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTCTRL, Value: 2},
				{Type: evdev.EV_KEY, Code: evdev.KEY_ESC, Value: 1},
			},
			wantBreak: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &modifierState{}
			gotBreak := false
			for _, ev := range tt.events {
				state.update(ev)
				if state.isBreakEvent(ev) {
					gotBreak = true
				}
			}
			if gotBreak != tt.wantBreak {
				t.Errorf("break detected = %v, want %v", gotBreak, tt.wantBreak)
			}
		})
	}
}
