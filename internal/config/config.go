package config

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Source says where a setting's effective value came from.
type Source string

const (
	FromDefault Source = "default"
	FromFile    Source = "file"
	FromFlag    Source = "flag"
)

// Config holds the effective value of every registered setting.
// The zero value is not usable; start from Defaults or Load.
type Config struct {
	values  map[string]string
	sources map[string]Source
}

// Path returns the config file location:
// <os.UserConfigDir()>/mergediff/config.toml.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate config directory: %w", err)
	}
	return filepath.Join(dir, "mergediff", "config.toml"), nil
}

// Defaults returns every setting at its default value.
func Defaults() Config {
	c := Config{values: map[string]string{}, sources: map[string]Source{}}
	for _, s := range Registry {
		c.values[s.Key] = s.Default
		c.sources[s.Key] = FromDefault
	}
	return c
}

// Load returns the defaults overridden by the file at path. A missing file
// is not an error.
func Load(path string) (Config, error) {
	c := Defaults()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return Config{}, err
	}
	file, err := Parse(data)
	if err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}
	for k, v := range file {
		c.values[k] = v
		c.sources[k] = FromFile
	}
	return c, nil
}

// Parse decodes a config file's contents and validates every key and value.
func Parse(data []byte) (map[string]string, error) {
	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		s, err := Lookup(k)
		if err != nil {
			return nil, err
		}
		str, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%w for %s: want a quoted string, got %v", ErrBadValue, k, v)
		}
		if err := s.Validate(str); err != nil {
			return nil, err
		}
		out[k] = str
	}
	return out, nil
}

// Override sets key to value with the given source, after validating it.
func (c *Config) Override(key, value string, src Source) error {
	if err := Check(key, value); err != nil {
		return err
	}
	c.values[key] = value
	c.sources[key] = src
	return nil
}

// Get returns the effective value of key ("" for an unknown key).
func (c Config) Get(key string) string { return c.values[key] }

// SourceOf returns where key's value came from.
func (c Config) SourceOf(key string) Source { return c.sources[key] }

// Values returns a copy of all effective values, keyed by setting key.
func (c Config) Values() map[string]string { return maps.Clone(c.values) }

// Theme is the theme setting: ThemeSystem, ThemeLight or ThemeDark.
func (c Config) Theme() string { return c.values[KeyTheme] }

// WindowState is the window-state setting: WindowNormal or WindowMaximized.
func (c Config) WindowState() string { return c.values[KeyWindowState] }
