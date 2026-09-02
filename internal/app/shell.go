package app

import (
	"context"

	"github.com/playdead/koa/internal/runner"
)

// Shell is the desktop-side handle on a Service. It exists so the Service
// itself carries only methods that are safe to bind to the frontend — Wails
// binds every exported method, and lifecycle hooks are not part of the API the
// UI should be able to call.
type Shell struct {
	*Service
}

// NewShell wraps a service for the desktop host.
func NewShell(s *Service) *Shell { return &Shell{Service: s} }

// Service returns the bindable API surface.
func (sh *Shell) API() *Service { return sh.Service }

// Startup connects the service to the running window.
func (sh *Shell) Startup(ctx context.Context, host Host) { sh.Service.start(ctx, host) }

// Shutdown stops child processes on quit.
func (sh *Shell) Shutdown() { sh.Service.stop() }

// Runner exposes the process manager for the tray and close handling.
func (sh *Shell) Runner() *runner.Manager { return sh.Service.procs }

// MinimizeToTray reports the close-behaviour setting (PRD §13).
func (sh *Shell) MinimizeToTray() bool { return sh.Service.store.Settings().MinimizeToTray }

// AppVersion is koa's own version string, for the tray and status footer.
func (sh *Shell) AppVersion() string { return sh.Service.version }
