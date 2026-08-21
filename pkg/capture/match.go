package capture

import (
	"path/filepath"
	"strings"
)

// Matcher filters a Device. Implementations are constructed via the
// helpers below (PathMatcher, NameMatcher, TypeMatcher).
type Matcher func(*Device) bool

// PathMatcher returns a Matcher that matches devices whose path
// equals or is a symlink-to one of the paths produced by globbing
// pattern. So /dev/input/by-id/usb-*-event-kbd will correctly resolve
// to /dev/input/eventN.
//
// An invalid glob (e.g. "[") produces a Matcher that matches nothing
// rather than failing at session-creation time, so partial configs
// still work.
func pathMatcher(pattern string) Matcher {
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return func(*Device) bool { return false }
	}

	// Pre-resolve symlinks once. If a symlink target disappears
	// mid-session we still want the matcher to behave predictably.
	resolved := make(map[string]string, len(matches))
	for _, m := range matches {
		if r, err := filepath.EvalSymlinks(m); err == nil {
			resolved[m] = r
		}
	}

	return func(d *Device) bool {
		for _, m := range matches {
			if d.Path == m {
				return true
			}
			if resolved[m] == d.Path {
				return true
			}
		}
		return false
	}
}

// NameMatcher returns a Matcher that matches devices whose Name
// contains substr (case-insensitive).
func nameMatcher(substr string) Matcher {
	needle := strings.ToLower(substr)
	return func(d *Device) bool {
		return strings.Contains(strings.ToLower(d.Name), needle)
	}
}

// typeMatcher matches if the device's detected types contain at least
// one of the requested types. A single type string is also parsed via
// ParseDeviceType so users can write "keyboard" or "mouse" without
// having to know the full enum.
func typeMatcher(types ...DeviceType) Matcher {
	return func(d *Device) bool {
		for _, want := range types {
			for _, have := range d.Types {
				if have == want {
					return true
				}
			}
		}
		return false
	}
}

// typeMatcherFromString parses a single user-supplied type string and
// produces a TypeMatcher. Unknown strings yield a no-match Matcher.
func typeMatcherFromString(s string) Matcher {
	t, err := ParseDeviceType(s)
	if err != nil {
		return func(*Device) bool { return false }
	}
	return typeMatcher(t)
}

// parseMatchExpr converts a single --match / --exclude expression
// ("key=value") into a Matcher. Supported keys:
//
//	path=…   glob match against the device path or its by-id alias
//	name=…   case-insensitive substring match against the device name
//	type=…   match a category (keyboard, mouse, touchpad, gamepad, other)
//
// Any other key produces an error so typos surface immediately rather
// than silently dropping the rule.
func parseMatchExpr(expr string) (Matcher, error) {
	k, v, ok := strings.Cut(expr, "=")
	if !ok {
		return nil, &MatchExprError{Expr: expr, Reason: "expected key=value"}
	}
	if v == "" {
		return nil, &MatchExprError{Expr: expr, Reason: "value is empty"}
	}

	switch strings.ToLower(k) {
	case "path":
		return pathMatcher(v), nil
	case "name":
		return nameMatcher(v), nil
	case "type":
		return typeMatcherFromString(v), nil
	default:
		return nil, &MatchExprError{
			Expr:   expr,
			Reason: "unknown key (want path, name, or type)",
		}
	}
}

// ParseMatchers is a convenience for the CLI: every expression is
// parsed and the first failure short-circuits with a joined error.
// Exported so the CLI can hand user-supplied strings straight in.
func ParseMatchers(exprs []string) ([]Matcher, error) {
	out := make([]Matcher, 0, len(exprs))
	for _, e := range exprs {
		m, err := parseMatchExpr(e)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// MatchExprError is returned when a --match / --exclude expression
// can't be parsed. Surfacing the offending expression makes CLI
// errors actionable.
type MatchExprError struct {
	Expr   string
	Reason string
}

func (e *MatchExprError) Error() string {
	return "invalid match expression " + strconvQuote(e.Expr) + ": " + e.Reason
}

// strconvQuote avoids importing strconv just for one call.
func strconvQuote(s string) string { return `"` + s + `"` }
