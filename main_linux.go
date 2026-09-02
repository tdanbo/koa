//go:build linux

package main

import "os"

// preventDoubleDecorations works around KDE's Wayland compositor not
// reliably honoring GTK's request for an undecorated window (Frameless in
// main.go), which leaves KWin's own title bar drawn alongside koa's custom
// one. Routing through XWayland sidesteps it: X11 window-manager hints for
// "no decorations" are respected consistently across window managers, unlike
// Wayland's per-compositor decoration negotiation. This must run before
// wails.Run() starts GTK.
func preventDoubleDecorations() {
	if _, set := os.LookupEnv("GDK_BACKEND"); !set {
		os.Setenv("GDK_BACKEND", "x11")
	}
}
