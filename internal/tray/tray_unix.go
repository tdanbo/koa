//go:build !windows

package tray

// platformIcon picks the PNG variant systray expects on Linux.
func platformIcon(icons Icons, dark bool) []byte {
	if dark {
		return icons.DarkPNG
	}
	return icons.LightPNG
}
