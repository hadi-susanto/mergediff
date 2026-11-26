package gui

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/hadi-susanto/mergediff/internal/config"
	"github.com/hadi-susanto/mergediff/internal/session"
)

func TestWindowOptions(t *testing.T) {
	sess := &session.Session{View: session.View{Title: "x.go"}}
	cfg := config.Defaults()
	o := windowOptions(sess, cfg)
	if o.URL != "/?theme=system" {
		t.Errorf("URL = %q", o.URL)
	}
	if o.Title != "mergediff — x.go" || o.StartState != application.WindowStateNormal ||
		o.Windows.Theme != application.SystemDefault || o.Mac.Appearance != "" {
		t.Errorf("defaults: %+v", o)
	}
	cfg.Override(config.KeyWindowState, config.WindowMaximized, config.FromFlag)
	cfg.Override(config.KeyTheme, config.ThemeDark, config.FromFlag)
	o = windowOptions(sess, cfg)
	if o.URL != "/?theme=dark" || o.StartState != application.WindowStateMaximised || o.Windows.Theme != application.Dark ||
		o.Mac.Appearance != application.NSAppearanceNameDarkAqua {
		t.Errorf("maximized dark: %+v", o)
	}
	cfg.Override(config.KeyTheme, config.ThemeLight, config.FromFlag)
	if o = windowOptions(sess, cfg); o.Windows.Theme != application.Light || o.Mac.Appearance != application.NSAppearanceNameAqua {
		t.Errorf("light: %+v", o)
	}
}
