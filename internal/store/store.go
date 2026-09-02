// Package store persists koa's local state — installed apps and settings — in
// a single JSON file in the OS config directory (PRD §16).
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// schemaVersion lets future releases migrate an older state.json.
const schemaVersion = 1

// Theme is the three-way appearance setting (PRD §15).
type Theme string

const (
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
	ThemeSystem Theme = "system"
)

// Settings are the user-facing preferences from the Settings panel (PRD §13).
type Settings struct {
	Theme          Theme `json:"theme"`
	MinimizeToTray bool  `json:"minimizeToTray"`
	// ManualOrgs are org names typed by hand. They matter only on the manual
	// token path, where GitHub returns no org memberships (PRD §7).
	ManualOrgs []string `json:"manualOrgs"`
	// TrustAcknowledged records that the user has seen the third-party binary
	// reminder (PRD §18).
	TrustAcknowledged bool `json:"trustAcknowledged"`
}

// App is one installed binary koa is tracking (PRD §10, §16).
type App struct {
	Owner       string    `json:"owner"`
	Repo        string    `json:"repo"`
	Description string    `json:"description"`
	Visibility  string    `json:"visibility"`
	Version     string    `json:"version"`
	AssetName   string    `json:"assetName"`
	BinaryPath  string    `json:"binaryPath"`
	SizeBytes   int64     `json:"sizeBytes"`
	InstalledAt time.Time `json:"installedAt"`
	PublishedAt time.Time `json:"publishedAt"`
	LastChecked time.Time `json:"lastChecked"`
	// LatestVersion is the newest tag seen by the last update check. Empty
	// until a check has run.
	LatestVersion string `json:"latestVersion"`
	AutoUpdate    bool   `json:"autoUpdate"`
}

// Key is the "owner/repo" identifier an app is stored under.
func (a App) Key() string { return Key(a.Owner, a.Repo) }

// HasUpdate reports whether the last check found a newer tag than the one on
// disk. Tags are compared literally — koa never parses version semantics (§9).
func (a App) HasUpdate() bool {
	return a.LatestVersion != "" && a.LatestVersion != a.Version
}

// Key builds the map key for an owner/repo pair.
func Key(owner, repo string) string {
	return strings.ToLower(owner) + "/" + strings.ToLower(repo)
}

// state is the on-disk document.
type state struct {
	SchemaVersion int            `json:"schemaVersion"`
	Settings      Settings       `json:"settings"`
	Apps          map[string]App `json:"apps"`
}

// Store is a concurrency-safe handle on the state file.
type Store struct {
	path string

	mu sync.RWMutex
	st state
}

// DefaultSettings are what a fresh install starts with: follow the system
// theme, quit on close (PRD §13, §15).
func DefaultSettings() Settings {
	return Settings{Theme: ThemeSystem, MinimizeToTray: false}
}

// Open loads the state file, creating an empty one in memory if it is absent.
// A corrupt file is reported rather than silently discarded.
func Open(path string) (*Store, error) {
	s := &Store{path: path, st: state{SchemaVersion: schemaVersion, Settings: DefaultSettings(), Apps: map[string]App{}}}

	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return s, nil
	}

	var loaded state
	if err := json.Unmarshal(raw, &loaded); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if loaded.Apps == nil {
		loaded.Apps = map[string]App{}
	}
	if loaded.Settings.Theme == "" {
		loaded.Settings.Theme = ThemeSystem
	}
	loaded.SchemaVersion = schemaVersion
	s.st = loaded
	return s, nil
}

// Path is the state file's location, for display in the status footer.
func (s *Store) Path() string { return s.path }

// Settings returns a copy of the current settings.
func (s *Store) Settings() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.st.Settings
	out.ManualOrgs = append([]string(nil), out.ManualOrgs...)
	return out
}

// UpdateSettings applies fn to the settings and persists the result.
func (s *Store) UpdateSettings(fn func(*Settings)) error {
	s.mu.Lock()
	fn(&s.st.Settings)
	s.mu.Unlock()
	return s.save()
}

// Apps returns every tracked app, sorted by repo name for stable UI ordering.
func (s *Store) Apps() []App {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]App, 0, len(s.st.Apps))
	for _, a := range s.st.Apps {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if strings.EqualFold(out[i].Repo, out[j].Repo) {
			return strings.ToLower(out[i].Owner) < strings.ToLower(out[j].Owner)
		}
		return strings.ToLower(out[i].Repo) < strings.ToLower(out[j].Repo)
	})
	return out
}

// App looks up a single tracked app.
func (s *Store) App(owner, repo string) (App, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.st.Apps[Key(owner, repo)]
	return a, ok
}

// PutApp inserts or replaces an app and persists.
func (s *Store) PutApp(a App) error {
	s.mu.Lock()
	s.st.Apps[a.Key()] = a
	s.mu.Unlock()
	return s.save()
}

// MutateApp applies fn to a tracked app and persists. It reports false when
// the app is not tracked, leaving the state untouched.
func (s *Store) MutateApp(owner, repo string, fn func(*App)) (bool, error) {
	s.mu.Lock()
	a, ok := s.st.Apps[Key(owner, repo)]
	if !ok {
		s.mu.Unlock()
		return false, nil
	}
	fn(&a)
	s.st.Apps[Key(owner, repo)] = a
	s.mu.Unlock()
	return true, s.save()
}

// DeleteApp forgets an app (PRD §10, uninstall).
func (s *Store) DeleteApp(owner, repo string) error {
	s.mu.Lock()
	delete(s.st.Apps, Key(owner, repo))
	s.mu.Unlock()
	return s.save()
}

// save writes the state file atomically: a sibling temp file followed by a
// rename, so an interrupted write can never truncate the real one.
func (s *Store) save() error {
	s.mu.RLock()
	raw, err := json.MarshalIndent(s.st, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".state-*.json")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp state file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp state file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("chmod temp state file: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace state file: %w", err)
	}
	return nil
}
