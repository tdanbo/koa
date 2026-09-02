//go:build windows

package tray

// platformIcon picks the ICO variant systray expects on Windows.
func platformIcon(icons Icons, dark bool) []byte {
	if dark {
		return icons.DarkICO
	}
	return icons.LightICO
}
