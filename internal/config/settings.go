// Package config holds mergediff's user settings: the registry of known
// settings, the TOML config file, and the layering of defaults, file and
// command-line values.
//
// To add a setting, add a Setting to Registry (and a typed accessor on
// Config if Go code reads it). Loading, validation, `mergediff settings`,
// command-line flags and the values sent to the frontend all follow.
package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Keys of the registered settings.
const (
	KeyTheme       = "theme"
	KeyWindowState = "window-state"
)

// Values of the registered settings that Go code checks.
const (
	ThemeSystem     = "system"
	ThemeLight      = "light"
	ThemeDark       = "dark"
	WindowNormal    = "normal"
	WindowMaximized = "maximized"
)

// Setting describes one user setting.
type Setting struct {
	Key     string   // key in config.toml and name of the --KEY flag
	Doc     string   // one-line description for `mergediff settings` and the usage text
	Default string   // value when neither the file nor a flag sets it
	Values  []string // allowed values
	Flag    bool     // also settable as --KEY VALUE on the command line
}

// Registry lists every setting, in display order.
var Registry = []Setting{
	{
		Key:     KeyTheme,
		Doc:     "colour theme; system follows the OS light/dark setting",
		Default: ThemeSystem,
		Values:  []string{ThemeSystem, ThemeLight, ThemeDark},
		Flag:    true,
	},
	{
		Key:     KeyWindowState,
		Doc:     "window state at startup",
		Default: WindowNormal,
		Values:  []string{WindowNormal, WindowMaximized},
		Flag:    true,
	},
}

var (
	// ErrUnknownKey is returned for a key that is not in Registry.
	ErrUnknownKey = errors.New("unknown setting")
	// ErrBadValue is returned for a value a setting does not allow.
	ErrBadValue = errors.New("invalid value")
)

// Lookup returns the setting named key.
func Lookup(key string) (Setting, error) {
	for _, s := range Registry {
		if s.Key == key {
			return s, nil
		}
	}
	return Setting{}, fmt.Errorf("%w %q (known: %s)", ErrUnknownKey, key, strings.Join(Keys(), ", "))
}

// Keys returns the keys of all settings, in registry order.
func Keys() []string {
	keys := make([]string, len(Registry))
	for i, s := range Registry {
		keys[i] = s.Key
	}
	return keys
}

// Validate checks that value is allowed for s.
func (s Setting) Validate(value string) error {
	if slices.Contains(s.Values, value) {
		return nil
	}
	return fmt.Errorf("%w %q for %s (allowed: %s)", ErrBadValue, value, s.Key, strings.Join(s.Values, ", "))
}

// Check validates key and value together.
func Check(key, value string) error {
	s, err := Lookup(key)
	if err != nil {
		return err
	}
	return s.Validate(value)
}
