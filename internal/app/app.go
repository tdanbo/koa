package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/playdead/koa/internal/assetmatch"
	"github.com/playdead/koa/internal/auth"
	"github.com/playdead/koa/internal/config"
	"github.com/playdead/koa/internal/ghapi"
	"github.com/playdead/koa/internal/installer"
	"github.com/playdead/koa/internal/markdown"
	"github.com/playdead/koa/internal/runner"
	"github.com/playdead/koa/internal/store"
)

// Topic is the GitHub topic maintainers add to opt a repo in (PRD §6).
const Topic = "koa"

// Host is the slice of the desktop shell the service needs. main.go supplies a
// Wails-backed implementation; tests supply a stub. Keeping it an interface is
// what lets this package compile and be tested without the GUI toolchain.
type Host interface {
	// Emit pushes an event to the frontend.
	Emit(event string, data any)
	// OpenURL opens a link in the user's default browser.
	OpenURL(url string) error
	// ShowWindow raises the main window, e.g. from the tray.
	ShowWindow()
	// Quit exits the application.
	Quit()
}

// noopHost is used before startup and in headless tests.
type noopHost struct{}

func (noopHost) Emit(string, any)     {}
func (noopHost) OpenURL(string) error { return nil }
func (noopHost) ShowWindow()          {}
func (noopHost) Quit()                {}

// Service implements every method the frontend can call.
type Service struct {
	version string
	paths   config.Paths
	store   *store.Store
	creds   *auth.CredentialStore
	gh      *ghapi.Client
	install *installer.Installer
	procs   *runner.Manager
	render  *markdown.Renderer
	device  *auth.DeviceFlow

	hostMu sync.RWMutex
	host   Host

	ctx    context.Context
	cancel context.CancelFunc

	accountMu sync.RWMutex
	account   Account

	// discoverMu guards the cached Discover result, which is reused when the
	// user navigates back to the view without asking for a refresh.
	discoverMu    sync.Mutex
	discoverCache *Discovery

	// releaseMu guards the per-repo latest-release cache.
	releaseMu    sync.Mutex
	releaseCache map[string]cachedRelease

	// signInMu guards the in-flight Device Flow attempt.
	signInMu     sync.Mutex
	signInCancel context.CancelFunc

	// startupError is surfaced once in Bootstrap.
	startupError string

	// selfUpdateOwner/selfUpdateName identify koa's own repository, so it can
	// check and install its own releases the same way it does for any other
	// koa-tagged repo. Both are empty when the running build has no
	// configured self-update target, which disables the feature entirely.
	selfUpdateOwner string
	selfUpdateName  string

	selfUpdateMu   sync.RWMutex
	selfUpdateInfo SelfUpdateInfo
	selfUpdating   atomic.Bool
}

// Options configures a Service.
type Options struct {
	Version  string
	ClientID string
	Paths    config.Paths
	// SelfUpdateRepo is koa's own "owner/repo" coordinate, used to check and
	// install koa's own releases. Empty disables self-update.
	SelfUpdateRepo string
}

// New wires the service together. It loads state and any stored credential,
// but performs no network calls.
func New(opts Options) (*Service, error) {
	if err := opts.Paths.EnsureDirs(); err != nil {
		return nil, err
	}

	st, err := store.Open(opts.Paths.StateFile)
	if err != nil {
		return nil, err
	}

	gh := ghapi.New("")
	selfOwner, selfName := parseSelfUpdateRepo(opts.SelfUpdateRepo)
	s := &Service{
		version: opts.Version,
		paths:   opts.Paths,
		store:   st,
		creds:   auth.NewCredentialStore(opts.Paths.ConfigDir),
		gh:      gh,
		install: installer.New(opts.Paths, gh),
		render:  markdown.NewRenderer(),
		device:  auth.NewDeviceFlow(opts.ClientID),
		host:    noopHost{},

		releaseCache: map[string]cachedRelease{},

		selfUpdateOwner: selfOwner,
		selfUpdateName:  selfName,
	}
	s.procs = runner.NewManager(func(event string, data any) { s.emit(event, data) })

	s.loadStoredCredential()
	return s, nil
}

// start is called by the Shell once the frontend exists.
func (s *Service) start(ctx context.Context, host Host) {
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.hostMu.Lock()
	s.host = host
	s.hostMu.Unlock()

	s.install.Sweep()
	sweepSelfUpdateLeftover()
	go s.ensurePath()
	go s.watchSelfUpdate(s.ctx)
}

// stop halts every child process so koa leaves nothing orphaned.
func (s *Service) stop() {
	if s.cancel != nil {
		s.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.procs.StopAll(ctx)
}

func (s *Service) emit(event string, data any) {
	s.hostMu.RLock()
	host := s.host
	s.hostMu.RUnlock()
	host.Emit(event, data)
}

// context returns the service context, falling back to Background before
// Startup has run (which is the case in tests).
func (s *Service) context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

// ensurePath makes sure the bin folder is on PATH, recording a non-fatal
// failure for the Bootstrap banner (PRD §6).
func (s *Service) ensurePath() {
	if _, err := ensurePathOnDisk(s.paths.BinDir); err != nil {
		s.startupError = fmt.Sprintf("koa could not add %s to your PATH: %v", config.Display(s.paths.BinDir), err)
	}
}

// loadStoredCredential restores a saved token and primes the account view.
func (s *Service) loadStoredCredential() {
	cred, err := s.creds.Load()
	if err != nil {
		if !errors.Is(err, auth.ErrNoCredential) {
			s.startupError = fmt.Sprintf("koa could not read the stored GitHub token: %v", err)
		}
		return
	}

	s.gh.SetToken(cred.Token)
	s.setAccount(Account{
		SignedIn:               true,
		Login:                  cred.Login,
		Source:                 string(cred.Source),
		Scopes:                 cred.Scopes,
		TokenStorage:           auth.StorageLabel(cred.Where),
		UsingPlaintextFallback: cred.Where == auth.StorageFile,
		PlaintextPath:          config.Display(s.creds.FallbackPath()),
	})
}

func (s *Service) setAccount(a Account) {
	s.accountMu.Lock()
	s.account = a
	s.accountMu.Unlock()
}

// Account returns the current account view.
func (s *Service) Account() Account {
	s.accountMu.RLock()
	defer s.accountMu.RUnlock()
	return s.account
}

// Bootstrap is the frontend's first call (PRD §5.2 shell, §5.5 gaps).
func (s *Service) Bootstrap() Bootstrap {
	settings := s.store.Settings()
	pathState := checkPath(s.paths.BinDir)

	return Bootstrap{
		Version:         s.version,
		Platform:        runtime.GOOS,
		Arch:            runtime.GOARCH,
		BinDir:          config.Display(s.paths.BinDir),
		BinDirAbsolute:  s.paths.BinDir,
		StateFile:       config.Display(s.paths.StateFile),
		Account:         s.Account(),
		Settings:        toSettingsView(settings),
		Path:            pathState,
		AssetPattern:    assetmatch.Pattern("{repo}", runtime.GOOS),
		DeviceFlowReady: s.device.ClientID != "",
		StartupError:    s.startupError,
	}
}

func toSettingsView(s store.Settings) Settings {
	orgs := s.ManualOrgs
	if orgs == nil {
		orgs = []string{}
	}
	return Settings{
		Theme:             string(s.Theme),
		MinimizeToTray:    s.MinimizeToTray,
		ManualOrgs:        orgs,
		TrustAcknowledged: s.TrustAcknowledged,
	}
}

// OpenExternal opens a URL in the user's browser. The frontend routes every
// outbound link through here rather than navigating the webview.
func (s *Service) OpenExternal(url string) error {
	s.hostMu.RLock()
	host := s.host
	s.hostMu.RUnlock()
	return host.OpenURL(url)
}

// RevealBinFolder opens the koa bin folder in the OS file manager (PRD §13).
func (s *Service) RevealBinFolder() error {
	return revealInFileManager(s.paths.BinDir)
}

// revealInFileManager opens dir with the platform's file browser.
func revealInFileManager(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	cmd := platformRevealCommand(dir)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open %s: %w", dir, err)
	}
	// explorer.exe returns a non-zero exit code even on success, so the
	// process is deliberately not waited on.
	go func() { _ = cmd.Wait() }()
	return nil
}
