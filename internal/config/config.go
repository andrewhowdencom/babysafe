// Package config defines the typed configuration for babysafe and the
// rules for hydrating it from viper. Keep this package free of any
// imports from internal/cli or internal/app — config is shared by both.
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config is the resolved runtime configuration. Keep this struct small;
// complex behavior should be inferred from these fields by the consumer.
type Config struct {
	Log     Log     `mapstructure:"log"`
	Capture Capture `mapstructure:"capture"`
}

// Log controls logging output. Currently only the level is exposed,
// but adding sinks / formats here is straightforward.
type Log struct {
	Level string `mapstructure:"level"`
}

// Capture holds knobs that govern how input devices are grabbed.
// `DeviceGlobs` are matched against /dev/input/event* paths.
type Capture struct {
	DeviceGlobs []string `mapstructure:"device-globs"`
	Excludes    []string `mapstructure:"excludes"`
	// ReleaseOnExit controls whether devices are released on clean exit.
	ReleaseOnExit bool `mapstructure:"release-on-exit"`
}

// NewDefault returns a Config populated with safe defaults.
func NewDefault() *Config {
	return &Config{
		Log: Log{
			Level: "info",
		},
		Capture: Capture{
			DeviceGlobs:   []string{"/dev/input/event*"},
			Excludes:      nil,
			ReleaseOnExit: true,
		},
	}
}

// All flattens the Config into a viper-friendly map of dotted keys.
// Keys must match the struct tags above; All is the source of truth
// for defaults.
func (c *Config) All() map[string]any {
	return map[string]any{
		"log.level":               c.Log.Level,
		"capture.device-globs":    c.Capture.DeviceGlobs,
		"capture.excludes":        c.Capture.Excludes,
		"capture.release-on-exit": c.Capture.ReleaseOnExit,
	}
}

// Hydrate reads dotted keys from v into the typed Config. Returns a
// helpful error if any unknown key is present — silently ignoring
// unknown keys is a long-term maintenance trap.
func (c *Config) Hydrate(v *viper.Viper) error {
	if err := v.Unmarshal(c); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}

	// Normalize / validate.
	c.Log.Level = strings.ToLower(strings.TrimSpace(c.Log.Level))
	switch c.Log.Level {
	case "", "debug", "info", "warn", "warning", "error":
	default:
		return fmt.Errorf("invalid log.level %q", c.Log.Level)
	}

	if len(c.Capture.DeviceGlobs) == 0 {
		c.Capture.DeviceGlobs = []string{"/dev/input/event*"}
	}

	return nil
}
