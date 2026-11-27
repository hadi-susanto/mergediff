package gui

import (
	"net/url"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/hadi-susanto/mergediff/internal/config"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// windowOptions configures a window from its session and the settings.
func windowOptions(sess session.Session, cfg config.Config) application.WebviewWindowOptions {
	opts := application.WebviewWindowOptions{
		Title:  "mergediff — " + sess.View().Title,
		Width:  1400,
		Height: 900,
		// The theme is in the URL so index.html can apply it before the
		// first paint; asking the backend would arrive a frame too late.
		URL: "/?theme=" + url.QueryEscape(cfg.Theme()),
	}
	if cfg.WindowState() == config.WindowMaximized {
		opts.StartState = application.WindowStateMaximised
	}
	// The page applies the theme itself (data-theme); these make the native
	// title bar and the first paint match. Linux has no such option: the
	// GTK title bar follows the system theme.
	switch cfg.Theme() {
	case config.ThemeDark:
		opts.Windows.Theme = application.Dark
		opts.Mac.Appearance = application.NSAppearanceNameDarkAqua
		opts.BackgroundColour = application.NewRGB(0x26, 0x26, 0x24) // --chrome-bg, dark
	case config.ThemeLight:
		opts.Windows.Theme = application.Light
		opts.Mac.Appearance = application.NSAppearanceNameAqua
		opts.BackgroundColour = application.NewRGB(0xf4, 0xf3, 0xef) // --chrome-bg, light
	}
	return opts
}
