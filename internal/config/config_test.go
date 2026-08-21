package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestNewDefault(t *testing.T) {
	c := NewDefault()

	if got := c.Log.Level; got != "info" {
		t.Errorf("default Log.Level = %q, want %q", got, "info")
	}
	if got := c.Capture.DeviceGlobs; len(got) != 1 || got[0] != "/dev/input/event*" {
		t.Errorf("default DeviceGlobs = %v, want [/dev/input/event*]", got)
	}
	if !c.Capture.ReleaseOnExit {
		t.Errorf("default ReleaseOnExit = false, want true")
	}
}

func TestHydrate(t *testing.T) {
	tests := []struct {
		name    string
		kv      map[string]any
		wantErr bool
		check   func(*testing.T, *Config)
	}{
		{
			name: "valid override",
			kv: map[string]any{
				"log.level":               "debug",
				"capture.device-globs":    []string{"/dev/input/event*", "/dev/input/by-id/foo"},
				"capture.excludes":        []string{"event1"},
				"capture.release-on-exit": false,
			},
			check: func(t *testing.T, c *Config) {
				t.Helper()
				if c.Log.Level != "debug" {
					t.Errorf("Level = %q, want debug", c.Log.Level)
				}
				if len(c.Capture.DeviceGlobs) != 2 {
					t.Errorf("DeviceGlobs len = %d, want 2", len(c.Capture.DeviceGlobs))
				}
				if c.Capture.ReleaseOnExit {
					t.Errorf("ReleaseOnExit = true, want false")
				}
			},
		},
		{
			name:    "invalid log level",
			kv:      map[string]any{"log.level": "LOUD"},
			wantErr: true,
		},
		{
			name: "empty globs falls back to default",
			kv:   map[string]any{"capture.device-globs": []string{}},
			check: func(t *testing.T, c *Config) {
				t.Helper()
				if len(c.Capture.DeviceGlobs) != 1 || c.Capture.DeviceGlobs[0] != "/dev/input/event*" {
					t.Errorf("DeviceGlobs = %v, want default", c.Capture.DeviceGlobs)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewDefault()
			v := viper.New()
			for k, val := range tt.kv {
				v.Set(k, val)
			}

			err := c.Hydrate(v)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Hydrate() err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.check != nil && err == nil {
				tt.check(t, c)
			}
		})
	}
}

func TestAll(t *testing.T) {
	c := NewDefault()
	all := c.All()

	// Every key must be the dotted form of a struct tag.
	want := []string{
		"log.level",
		"capture.device-globs",
		"capture.excludes",
		"capture.release-on-exit",
	}
	for _, k := range want {
		if _, ok := all[k]; !ok {
			t.Errorf("All() missing key %q", k)
		}
	}
	if len(all) != len(want) {
		t.Errorf("All() returned %d keys, want %d (%v)", len(all), len(want), all)
	}

	// Sanity check that values round-trip through Hydrate.
	c2 := NewDefault()
	v := viper.New()
	for k, val := range all {
		v.Set(k, val)
	}
	if err := c2.Hydrate(v); err != nil {
		t.Fatalf("Hydrate round-trip failed: %v", err)
	}
	if !strings.EqualFold(c2.Log.Level, c.Log.Level) {
		t.Errorf("round-trip Log.Level = %q, want %q", c2.Log.Level, c.Log.Level)
	}
}
