package command

import (
	"github.com/hadi-susanto/mergediff/internal/cliargs"
	"github.com/hadi-susanto/mergediff/internal/config"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// openWindow loads the settings and opens the window for sess, returning
// the window's exit code. The config is read only here, so commands that
// open no window work with a broken config file.
func openWindow(env Env, sess *session.Session, flags []cliargs.Setting) (int, error) {
	cfg, err := loadSettings(env, flags)
	if err != nil {
		return ExitError, err
	}
	return env.OpenWindow(sess, cfg), nil
}

// loadSettings returns the config file's settings overridden by the
// command-line flags. Without a config directory (e.g. no home directory)
// the defaults apply: settings are optional and must not block a merge.
func loadSettings(env Env, flags []cliargs.Setting) (config.Config, error) {
	cfg := config.Defaults()
	if path, err := env.ConfigPath(); err == nil {
		if cfg, err = config.Load(path); err != nil {
			return config.Config{}, err
		}
	}
	for _, f := range flags {
		if err := cfg.Override(f.Key, f.Value, config.FromFlag); err != nil {
			return config.Config{}, err // cliargs already validated; a registry bug
		}
	}
	return cfg, nil
}
