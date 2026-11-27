package gui

// mainWindowName names the window opened from the command line.
const mainWindowName = "main"

// closeRequestedEvent names the event that tells a window's frontend its
// close button was used. Events reach every window, so the name carries
// the window's name and each window listens only for its own.
func closeRequestedEvent(window string) string {
	return "mergediff:close-requested:" + window
}

// conflictsChangedEvent names the event that tells a Conflicts window
// (window parent) to refresh: one of its merge windows opened or closed.
func conflictsChangedEvent(parent string) string {
	return "mergediff:conflicts-changed:" + parent
}
