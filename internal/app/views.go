// Package app is koa's application service: the API surface the Wails frontend
// binds to, and the view models it renders. Everything the UI needs is shaped
// here so the frontend holds presentation only.
package app

import (
	"time"

	"github.com/playdead/koa/internal/runner"
)

// StatusKind classifies a repo's install state so the frontend can colour the
// status text semantically — sage for installed, amber for updates and
// incompatibility, neutral for not installed (PRD §5.4).
type StatusKind string

const (
	StatusNotInstalled StatusKind = "none"
	StatusInstalled    StatusKind = "installed"
	StatusUpdate       StatusKind = "update"
	StatusIncompatible StatusKind = "incompatible"
	// StatusNoRelease covers a koa-tagged repo that has published nothing yet.
	StatusNoRelease StatusKind = "norelease"
)

// Account describes the signed-in GitHub user (PRD §5.2, §13).
type Account struct {
	SignedIn  bool   `json:"signedIn"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	// Source is "device" or "manual", which changes what Settings explains
	// about org detection (PRD §7).
	Source string `json:"source"`
	Scopes string `json:"scopes"`
	// TokenStorage is a human phrase such as "Windows Credential Manager".
	TokenStorage string `json:"tokenStorage"`
	// UsingPlaintextFallback flags that no keyring was available (PRD §4).
	UsingPlaintextFallback bool   `json:"usingPlaintextFallback"`
	PlaintextPath          string `json:"plaintextPath"`
}

// Settings mirrors store.Settings for the frontend.
type Settings struct {
	Theme             string   `json:"theme"`
	MinimizeToTray    bool     `json:"minimizeToTray"`
	ManualOrgs        []string `json:"manualOrgs"`
	TrustAcknowledged bool     `json:"trustAcknowledged"`
}

// PathState tells the user whether the bin folder is actually on PATH, which
// the status footer reports (PRD §5.2).
type PathState struct {
	OnPath       bool   `json:"onPath"`
	Persisted    bool   `json:"persisted"`
	NeedsRestart bool   `json:"needsRestart"`
	Detail       string `json:"detail"`
}

// Bootstrap is the single payload the frontend fetches on startup.
type Bootstrap struct {
	Version  string `json:"version"`
	Platform string `json:"platform"`
	Arch     string `json:"arch"`
	// BinDir and StateFile are in display form (`~/.koa/bin`), with the
	// absolute variants alongside for reveal actions (PRD §5.5).
	BinDir          string    `json:"binDir"`
	BinDirAbsolute  string    `json:"binDirAbsolute"`
	StateFile       string    `json:"stateFile"`
	Account         Account   `json:"account"`
	Settings        Settings  `json:"settings"`
	Path            PathState `json:"path"`
	AssetPattern    string    `json:"assetPattern"`
	DeviceFlowReady bool      `json:"deviceFlowReady"`
	// StartupError is a non-fatal problem worth surfacing as a banner, such as
	// a failed PATH write.
	StartupError string `json:"startupError"`
}

// Repo is one row in Discover and the header of Repo detail (PRD §8).
type Repo struct {
	ID          string `json:"id"`
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
	HTMLURL     string `json:"htmlUrl"`
	// OwnerScope is "you" when the repo is under the signed-in account, or the
	// org login otherwise, so the badge can say which org it came from.
	OwnerScope string `json:"ownerScope"`

	Status     string     `json:"status"`
	StatusKind StatusKind `json:"statusKind"`
	// Action is the button label: Install, Update, Open, or empty when the
	// repo cannot be acted on.
	Action           string `json:"action"`
	CanInstall       bool   `json:"canInstall"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installedVersion"`

	Incompatible       bool   `json:"incompatible"`
	IncompatibleReason string `json:"incompatibleReason"`

	LatestVersion string    `json:"latestVersion"`
	PublishedAt   time.Time `json:"publishedAt"`
	AssetName     string    `json:"assetName"`
	AssetSize     int64     `json:"assetSize"`
}

// ScopeError is a per-scope discovery failure, so one broken org does not
// blank the whole view (PRD §7 SSO, §5.5 error states).
type ScopeError struct {
	Scope   string `json:"scope"`
	Message string `json:"message"`
	SSO     bool   `json:"sso"`
	SSOURL  string `json:"ssoUrl"`
}

// Discovery is the Discover view's payload.
type Discovery struct {
	Repos       []Repo       `json:"repos"`
	Scopes      []string     `json:"scopes"`
	Errors      []ScopeError `json:"errors"`
	RefreshedAt time.Time    `json:"refreshedAt"`
	SignedIn    bool         `json:"signedIn"`
}

// RepoDetail is the Repo detail view (PRD §8, §12).
type RepoDetail struct {
	Repo        Repo   `json:"repo"`
	ReadmeHTML  string `json:"readmeHtml"`
	ReadmeError string `json:"readmeError"`
}

// App is one installed binary, for the Installed list and App detail header.
type App struct {
	ID          string `json:"id"`
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`

	Version       string    `json:"version"`
	LatestVersion string    `json:"latestVersion"`
	HasUpdate     bool      `json:"hasUpdate"`
	Running       bool      `json:"running"`
	AutoUpdate    bool      `json:"autoUpdate"`
	LastChecked   time.Time `json:"lastChecked"`
	InstalledAt   time.Time `json:"installedAt"`
	PublishedAt   time.Time `json:"publishedAt"`

	// BinaryPath is display form; Command is what the user types in a terminal.
	BinaryPath         string `json:"binaryPath"`
	BinaryPathAbsolute string `json:"binaryPathAbsolute"`
	Command            string `json:"command"`
	AssetName          string `json:"assetName"`
	SizeBytes          int64  `json:"sizeBytes"`
	// Missing is true when state tracks the app but the binary is gone from
	// the bin folder.
	Missing bool `json:"missing"`
}

// AppDetail is the App detail view.
type AppDetail struct {
	App               App    `json:"app"`
	LatestPublishedAt string `json:"latestPublishedAt"`
	ReadmeHTML        string `json:"readmeHtml"`
	ReadmeError       string `json:"readmeError"`
}

// Version is one row of the Versions tab (PRD §10).
type Version struct {
	Tag         string    `json:"tag"`
	PublishedAt time.Time `json:"publishedAt"`
	SizeBytes   int64     `json:"sizeBytes"`
	IsCurrent   bool      `json:"isCurrent"`
	IsLatest    bool      `json:"isLatest"`
	Compatible  bool      `json:"compatible"`
	// Action is Reinstall for the current tag, Update for a newer one, and
	// Roll back for anything older.
	Action string `json:"action"`
}

// InstallProgress is streamed during an install so the UI can show a stage and
// a byte count (PRD §5.5).
type InstallProgress struct {
	ID    string `json:"id"`
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Tag   string `json:"tag"`
	Stage string `json:"stage"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
	Error string `json:"error"`
}

// SignInPrompt is what the sign-in screen displays while Device Flow runs
// (PRD §5.5, §7).
type SignInPrompt struct {
	UserCode        string    `json:"userCode"`
	VerificationURI string    `json:"verificationUri"`
	ExpiresAt       time.Time `json:"expiresAt"`
	// BrowserOpened reports whether koa managed to open the browser itself.
	BrowserOpened bool `json:"browserOpened"`
}

// AuthEvent is emitted when a Device Flow attempt finishes.
type AuthEvent struct {
	Status  string  `json:"status"` // "signed-in" | "failed" | "cancelled"
	Account Account `json:"account"`
	Error   string  `json:"error"`
}

// Process re-exports the runner's view so the frontend has one import surface.
type Process = runner.Process

// LogLine re-exports a runner log line.
type LogLine = runner.Line

// Event names emitted to the frontend.
const (
	EventAuth       = "koa:auth"
	EventInstall    = "koa:install"
	EventApps       = "koa:apps"
	EventToast      = "koa:toast"
	EventSelfUpdate = "koa:selfupdate"
)

// Toast is a transient message shown for background outcomes such as an
// auto-update completing.
type Toast struct {
	Kind    string `json:"kind"` // "info" | "success" | "warning" | "error"
	Message string `json:"message"`
}
