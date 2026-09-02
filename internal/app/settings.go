package app

import (
	"errors"
	"sort"
	"strings"

	"github.com/playdead/koa/internal/store"
)

// GetSettings returns the current preferences (PRD §13).
func (s *Service) GetSettings() Settings {
	return toSettingsView(s.store.Settings())
}

// SetTheme stores the Light / Dark / System choice (PRD §15).
func (s *Service) SetTheme(theme string) (Settings, error) {
	value := store.Theme(strings.ToLower(strings.TrimSpace(theme)))
	switch value {
	case store.ThemeLight, store.ThemeDark, store.ThemeSystem:
	default:
		return Settings{}, errors.New("theme must be light, dark or system")
	}
	if err := s.store.UpdateSettings(func(cfg *store.Settings) { cfg.Theme = value }); err != nil {
		return Settings{}, err
	}
	return s.GetSettings(), nil
}

// SetMinimizeToTray controls whether closing the window hides koa to the tray
// or quits it (PRD §13, §14).
func (s *Service) SetMinimizeToTray(enabled bool) (Settings, error) {
	if err := s.store.UpdateSettings(func(cfg *store.Settings) { cfg.MinimizeToTray = enabled }); err != nil {
		return Settings{}, err
	}
	return s.GetSettings(), nil
}

// SetManualOrgs records org names typed by hand. They only matter on the manual
// token path, where GitHub reports no org memberships (PRD §7, §16).
func (s *Service) SetManualOrgs(orgs []string) (Settings, error) {
	cleaned := make([]string, 0, len(orgs))
	seen := map[string]bool{}
	for _, o := range orgs {
		o = strings.TrimSpace(o)
		if o == "" || seen[strings.ToLower(o)] {
			continue
		}
		seen[strings.ToLower(o)] = true
		cleaned = append(cleaned, o)
	}
	sort.Slice(cleaned, func(i, j int) bool { return strings.ToLower(cleaned[i]) < strings.ToLower(cleaned[j]) })

	if err := s.store.UpdateSettings(func(cfg *store.Settings) { cfg.ManualOrgs = cleaned }); err != nil {
		return Settings{}, err
	}
	s.invalidateDiscovery()
	return s.GetSettings(), nil
}

// AcknowledgeTrust records that the user has seen the third-party binary
// reminder, so it is shown once rather than before every install (PRD §18).
func (s *Service) AcknowledgeTrust() (Settings, error) {
	if err := s.store.UpdateSettings(func(cfg *store.Settings) { cfg.TrustAcknowledged = true }); err != nil {
		return Settings{}, err
	}
	return s.GetSettings(), nil
}

// EnsurePath re-runs the PATH setup, for a Settings-level retry after a
// failure (PRD §6).
func (s *Service) EnsurePath() (PathState, error) {
	return ensurePathOnDisk(s.paths.BinDir)
}

// PathStatus reports whether the bin folder is currently on PATH.
func (s *Service) PathStatus() PathState { return checkPath(s.paths.BinDir) }
