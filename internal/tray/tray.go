// Package tray puts koa's "K" monogram in the system tray (PRD §14).
package tray

import (
	"sync"

	"fyne.io/systray"
)

// Icons holds the two monogram variants. Light is a dark glyph for light tray
// backgrounds; Dark is a light glyph for dark ones (PRD §14).
type Icons struct {
	LightPNG []byte
	DarkPNG  []byte
	LightICO []byte
	DarkICO  []byte
}

// Options configures the tray.
type Options struct {
	Title   string
	Tooltip string
	Icons   Icons

	// OnOpen raises the main window.
	OnOpen func()
	// OnCheckUpdates runs a refresh-all in the background.
	OnCheckUpdates func()
	// OnQuit exits koa.
	OnQuit func()
}

// Tray is a running tray presence.
type Tray struct {
	icons Icons

	mu      sync.Mutex
	started bool
	stopped bool
	// wantDark remembers the requested variant so a call that arrives before
	// the tray is ready is applied once it is.
	wantDark bool
}

// Start puts the icon in the tray and returns immediately. The tray runs on
// its own goroutine; systray's Linux backend speaks DBus and its Windows
// backend owns a hidden message window, so neither needs the main thread.
func Start(opts Options) *Tray {
	t := &Tray{icons: opts.Icons, wantDark: true}

	onReady := func() {
		t.mu.Lock()
		t.started = true
		dark := t.wantDark
		t.mu.Unlock()

		systray.SetTitle(opts.Title)
		systray.SetTooltip(opts.Tooltip)
		t.applyIcon(dark)

		open := systray.AddMenuItem("Open koa", "Bring the koa window to the front")
		check := systray.AddMenuItem("Check for updates", "Check every installed app for a newer release")
		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit", "Exit koa")

		go func() {
			for {
				select {
				case <-open.ClickedCh:
					invoke(opts.OnOpen)
				case <-check.ClickedCh:
					go invoke(opts.OnCheckUpdates)
				case <-quit.ClickedCh:
					invoke(opts.OnQuit)
					return
				}
			}
		}()
	}

	go systray.Run(onReady, func() {})
	return t
}

// SetDarkBackground picks the monogram variant that will read against the
// current tray background.
func (t *Tray) SetDarkBackground(dark bool) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.wantDark = dark
	ready := t.started && !t.stopped
	t.mu.Unlock()
	if ready {
		t.applyIcon(dark)
	}
}

func (t *Tray) applyIcon(dark bool) {
	if icon := t.icon(dark); len(icon) > 0 {
		systray.SetIcon(icon)
	}
}

func (t *Tray) icon(dark bool) []byte {
	return platformIcon(t.icons, dark)
}

// Stop removes the icon from the tray.
func (t *Tray) Stop() {
	if t == nil {
		return
	}
	t.mu.Lock()
	if t.stopped || !t.started {
		t.stopped = true
		t.mu.Unlock()
		return
	}
	t.stopped = true
	t.mu.Unlock()
	systray.Quit()
}

func invoke(fn func()) {
	if fn != nil {
		fn()
	}
}
