// Command koa is a desktop installer for binaries published as GitHub releases
// by repositories carrying the `koa` topic. See PRD/PRD.md.
package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/playdead/koa/internal/app"
	"github.com/playdead/koa/internal/config"
	"github.com/playdead/koa/internal/store"
	"github.com/playdead/koa/internal/tray"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/tray-dark.png build/tray-light.png build/tray-dark.ico build/tray-light.ico
var icons embed.FS

//go:embed build/appicon.png
var appIconPNG []byte

// version, githubClientID and selfUpdateRepo are set at build time:
//
//	go build -ldflags "-X main.version=1.0.0 -X main.githubClientID=Iv1... -X main.selfUpdateRepo=owner/koa"
//
// The Client ID is safe to embed: Device Flow uses no client secret, and koa is
// open source (PRD §7). selfUpdateRepo is the "owner/repo" koa checks and
// installs its own releases from; the release workflow injects the repo that
// actually built the binary, so a fork self-updates from itself rather than
// upstream.
var (
	version        = "dev"
	githubClientID = ""
	selfUpdateRepo = ""
)

// windowBackground matches the reference's window surface so there is no flash
// of white before the frontend paints (PRD §5.4).
var windowBackground = &options.RGBA{R: 0x0d, G: 0x0e, B: 0x0f, A: 255}

func main() {
	preventDoubleDecorations()

	paths, err := config.Resolve()
	if err != nil {
		log.Fatalf("koa: %v", err)
	}

	service, err := app.New(app.Options{
		Version:        version,
		ClientID:       clientID(),
		Paths:          paths,
		SelfUpdateRepo: selfUpdateRepoValue(),
	})
	if err != nil {
		log.Fatalf("koa: %v", err)
	}
	shell := app.NewShell(service)

	desktop := &desktop{shell: shell}
	bound := &Koa{Service: service}
	windowAPI := &Window{desktop: desktop}

	err = wails.Run(&options.App{
		Title:  "Koa",
		Width:  1180,
		Height: 760,
		// The reference's three-band layout needs room; below this the rail
		// and the 82px action column stop fitting (PRD §5.5).
		MinWidth:         920,
		MinHeight:        600,
		Frameless:        true,
		BackgroundColour: windowBackground,
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        desktop.startup,
		OnBeforeClose:    desktop.beforeClose,
		OnShutdown:       desktop.shutdown,
		Bind:             []any{bound, windowAPI},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		Linux: &linux.Options{
			Icon:                appIconPNG,
			WindowIsTranslucent: false,
			ProgramName:         "koa",
		},
	})
	if err != nil {
		log.Fatalf("koa: %v", err)
	}
}

// clientID prefers the environment, so a developer can run against their own
// OAuth App without rebuilding (PRD §19).
func clientID() string {
	if fromEnv := strings.TrimSpace(os.Getenv("KOA_GITHUB_CLIENT_ID")); fromEnv != "" {
		return fromEnv
	}
	return strings.TrimSpace(githubClientID)
}

// selfUpdateRepoValue prefers the environment, so a developer can point a
// local build at a fork's releases without rebuilding.
func selfUpdateRepoValue() string {
	if fromEnv := strings.TrimSpace(os.Getenv("KOA_SELF_UPDATE_REPO")); fromEnv != "" {
		return fromEnv
	}
	return strings.TrimSpace(selfUpdateRepo)
}

// Koa is the API surface bound to the frontend. Only the service's methods are
// promoted onto it, which keeps lifecycle hooks out of the JS bridge.
type Koa struct {
	*app.Service
}

// desktop owns the window, the tray, and the close behaviour.
type desktop struct {
	shell *app.Shell
	ctx   context.Context
	tray  *tray.Tray
}

func (d *desktop) startup(ctx context.Context) {
	d.ctx = ctx
	d.shell.Startup(ctx, &wailsHost{ctx: ctx})
	d.tray = tray.Start(tray.Options{
		Title:          "Koa",
		Tooltip:        "Koa " + d.shell.AppVersion(),
		Icons:          loadIcons(),
		OnOpen:         d.show,
		OnCheckUpdates: d.checkUpdatesFromTray,
		OnQuit:         d.quit,
	})
	d.syncTrayIcon()
}

// beforeClose implements the close-behaviour toggle: hide to the tray when the
// setting is on, quit otherwise (PRD §13, §14).
func (d *desktop) beforeClose(ctx context.Context) bool {
	if d.shell.MinimizeToTray() {
		wruntime.WindowHide(ctx)
		return true
	}
	return false
}

func (d *desktop) shutdown(context.Context) {
	d.tray.Stop()
	d.shell.Shutdown()
}

func (d *desktop) show() {
	if d.ctx == nil {
		return
	}
	wruntime.WindowShow(d.ctx)
	wruntime.WindowUnminimise(d.ctx)
}

func (d *desktop) quit() {
	if d.ctx == nil {
		return
	}
	wruntime.Quit(d.ctx)
}

// checkUpdatesFromTray runs the same refresh as the Installed view's button and
// reports the outcome through the normal toast channel.
func (d *desktop) checkUpdatesFromTray() {
	apps, err := d.shell.CheckAllForUpdates()
	if err != nil {
		return
	}
	pending := 0
	for _, a := range apps {
		if a.HasUpdate {
			pending++
		}
	}
	message := "Everything is up to date."
	if pending == 1 {
		message = "1 update available."
	} else if pending > 1 {
		message = fmt.Sprintf("%d updates available.", pending)
	}
	if d.ctx != nil {
		wruntime.EventsEmit(d.ctx, app.EventToast, app.Toast{Kind: "info", Message: message})
	}
}

// syncTrayIcon picks the monogram variant matching the user's theme, which is
// the best signal koa has for the tray background (PRD §14).
func (d *desktop) syncTrayIcon() {
	d.tray.SetDarkBackground(d.shell.GetSettings().Theme != string(store.ThemeLight))
}

func loadIcons() tray.Icons {
	read := func(name string) []byte {
		body, err := icons.ReadFile(name)
		if err != nil {
			return nil
		}
		return body
	}
	return tray.Icons{
		LightPNG: read("build/tray-light.png"),
		DarkPNG:  read("build/tray-dark.png"),
		LightICO: read("build/tray-light.ico"),
		DarkICO:  read("build/tray-dark.ico"),
	}
}

// Window is the bound API for koa's custom title bar, which draws its own
// chrome because the window is frameless (PRD §5.2).
type Window struct {
	desktop *desktop
}

// Minimise minimises the window.
func (w *Window) Minimise() {
	if w.desktop.ctx == nil {
		return
	}
	wruntime.WindowMinimise(w.desktop.ctx)
}

// ToggleMaximise maximises or restores the window.
func (w *Window) ToggleMaximise() {
	if w.desktop.ctx == nil {
		return
	}
	wruntime.WindowToggleMaximise(w.desktop.ctx)
}

// Close routes the title bar's close button through the same behaviour as the
// OS close button, so the tray setting applies to both.
func (w *Window) Close() {
	if w.desktop.ctx == nil {
		return
	}
	if w.desktop.shell.MinimizeToTray() {
		wruntime.WindowHide(w.desktop.ctx)
		return
	}
	wruntime.Quit(w.desktop.ctx)
}

// SyncTrayIcon lets the frontend tell the tray that the theme changed.
func (w *Window) SyncTrayIcon() {
	w.desktop.syncTrayIcon()
}

// wailsHost adapts the Wails runtime to the service's Host interface.
type wailsHost struct {
	ctx context.Context
}

func (h *wailsHost) Emit(event string, data any) {
	wruntime.EventsEmit(h.ctx, event, data)
}

func (h *wailsHost) OpenURL(url string) error {
	wruntime.BrowserOpenURL(h.ctx, url)
	return nil
}

func (h *wailsHost) ShowWindow() {
	wruntime.WindowShow(h.ctx)
	wruntime.WindowUnminimise(h.ctx)
}

func (h *wailsHost) Quit() {
	wruntime.Quit(h.ctx)
}
