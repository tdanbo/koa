package app

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/playdead/koa/internal/assetmatch"
	"github.com/playdead/koa/internal/config"
	"github.com/playdead/koa/internal/ghapi"
	"github.com/playdead/koa/internal/installer"
	"github.com/playdead/koa/internal/store"
)

// RepoDetail is the pre-install detail view: header facts plus the readme,
// fetched from the readme endpoint rather than a release archive (PRD §8, §12).
func (s *Service) RepoDetail(owner, name string) (RepoDetail, error) {
	if !s.Account().SignedIn {
		return RepoDetail{}, errors.New("sign in with GitHub to browse repositories")
	}

	ctx := s.context()
	repo, err := s.gh.Repository(ctx, owner, name)
	if err != nil {
		return RepoDetail{}, err
	}

	detail := RepoDetail{Repo: s.repoView(ctx, repo, s.Account())}

	html, err := s.renderReadme(ctx, repo)
	if err != nil {
		detail.ReadmeError = readmeErrorMessage(err)
	}
	detail.ReadmeHTML = html
	return detail, nil
}

// Readme renders a repo's readme on demand, for the App detail Readme tab.
func (s *Service) Readme(owner, name string) (string, error) {
	ctx := s.context()
	repo, err := s.gh.Repository(ctx, owner, name)
	if err != nil {
		return "", err
	}
	return s.renderReadme(ctx, repo)
}

func (s *Service) renderReadme(ctx context.Context, repo ghapi.Repo) (string, error) {
	md, err := s.gh.Readme(ctx, repo.Owner.Login, repo.Name)
	if err != nil {
		return "", err
	}
	return s.render.Render(md, repo.Owner.Login, repo.Name, repo.DefaultBranch)
}

func readmeErrorMessage(err error) string {
	if ghapi.IsNotFound(err) {
		return "This repository has no readme."
	}
	return err.Error()
}

// Installed lists every tracked app for the Installed view (PRD §10).
func (s *Service) Installed() []App {
	running := s.runningKeys()
	apps := s.store.Apps()
	out := make([]App, 0, len(apps))
	for _, a := range apps {
		out = append(out, s.appView(a, running[a.Key()]))
	}
	return out
}

func (s *Service) appView(a store.App, running bool) App {
	absolute := a.BinaryPath
	if absolute == "" {
		absolute = s.paths.BinaryPath(a.Repo)
	}
	size := s.install.BinarySize(a.Repo)

	return App{
		ID:                 a.Key(),
		Owner:              a.Owner,
		Name:               a.Repo,
		Description:        a.Description,
		Visibility:         a.Visibility,
		Version:            a.Version,
		LatestVersion:      a.LatestVersion,
		HasUpdate:          a.HasUpdate(),
		Running:            running,
		AutoUpdate:         a.AutoUpdate,
		LastChecked:        a.LastChecked,
		InstalledAt:        a.InstalledAt,
		PublishedAt:        a.PublishedAt,
		BinaryPath:         config.Display(absolute),
		BinaryPathAbsolute: absolute,
		Command:            config.CommandName(a.Repo),
		AssetName:          a.AssetName,
		SizeBytes:          size,
		Missing:            size == 0,
	}
}

// AppDetail is the installed-app detail view.
func (s *Service) AppDetail(owner, name string) (AppDetail, error) {
	tracked, ok := s.store.App(owner, name)
	if !ok {
		return AppDetail{}, fmt.Errorf("%s/%s is not installed", owner, name)
	}

	detail := AppDetail{App: s.appView(tracked, s.runningKeys()[tracked.Key()])}
	if release, err := s.latestRelease(s.context(), owner, name); err == nil {
		detail.LatestPublishedAt = release.PublishedAt.Format(time.RFC3339)
	}
	return detail, nil
}

// Versions lists the release tags an installed app can be moved between.
// Only one version lives on disk at a time; selecting a tag replaces it
// (PRD §10).
func (s *Service) Versions(owner, name string) ([]Version, error) {
	releases, err := s.gh.ListReleases(s.context(), owner, name, 30)
	if err != nil {
		return nil, err
	}

	current := ""
	if tracked, ok := s.store.App(owner, name); ok {
		current = tracked.Version
	}

	out := make([]Version, 0, len(releases))
	for i, r := range releases {
		v := Version{
			Tag:         r.TagName,
			PublishedAt: r.PublishedAt,
			IsCurrent:   r.TagName == current,
			IsLatest:    i == 0,
		}
		if match, ok := assetmatch.Pick(name, r.AssetNames()); ok {
			v.Compatible = true
			if asset, found := r.AssetByName(match.Filename); found {
				v.SizeBytes = asset.Size
			}
		}
		switch {
		case v.IsCurrent:
			v.Action = "Reinstall"
		case v.IsLatest:
			v.Action = "Update"
		default:
			v.Action = "Roll back"
		}
		out = append(out, v)
	}
	return out, nil
}

// Install downloads and installs a repo's binary. An empty tag means the
// latest release; a specific tag drives reinstall and rollback (PRD §10).
func (s *Service) Install(owner, name, tag string) (App, error) {
	if !s.Account().SignedIn {
		return App{}, errors.New("sign in with GitHub to install")
	}

	ctx := s.context()
	id := store.Key(owner, name)

	emit := func(stage string, done, total int64, errText string) {
		s.emit(EventInstall, InstallProgress{
			ID: id, Owner: owner, Repo: name, Tag: tag,
			Stage: stage, Done: done, Total: total, Error: errText,
		})
	}

	result, err := s.install.Install(ctx, installer.Request{Owner: owner, Repo: name, Tag: tag},
		func(stage installer.Stage, done, total int64) {
			emit(string(stage), done, total, "")
		})
	if err != nil {
		emit("failed", 0, 0, installErrorMessage(err))
		return App{}, errors.New(installErrorMessage(err))
	}

	// Carry over metadata so the Installed list has a description without a
	// second round-trip on every render.
	existing, _ := s.store.App(owner, name)
	record := store.App{
		Owner:         owner,
		Repo:          name,
		Description:   existing.Description,
		Visibility:    existing.Visibility,
		Version:       result.Tag,
		AssetName:     result.AssetName,
		BinaryPath:    result.BinaryPath,
		SizeBytes:     result.SizeBytes,
		InstalledAt:   time.Now(),
		PublishedAt:   result.PublishedAt,
		LastChecked:   time.Now(),
		LatestVersion: result.Tag,
		AutoUpdate:    existing.AutoUpdate,
	}
	if repo, err := s.gh.Repository(ctx, owner, name); err == nil {
		record.Description = repo.Description
		record.Visibility = repo.Visibility()
	}
	// A rollback leaves a newer release available; reflect that immediately.
	if latest, err := s.latestRelease(ctx, owner, name); err == nil {
		record.LatestVersion = latest.TagName
	}

	if err := s.store.PutApp(record); err != nil {
		return App{}, err
	}

	view := s.appView(record, false)
	s.emit(EventApps, s.Installed())
	return view, nil
}

// installErrorMessage turns installer failures into sentences the UI can show
// verbatim.
func installErrorMessage(err error) string {
	var incompat *installer.ErrIncompatible
	if errors.As(err, &incompat) {
		return fmt.Sprintf("Release %s publishes no asset matching %s.", incompat.Tag, incompat.Pattern)
	}
	if errors.Is(err, ghapi.ErrNoReleases) {
		return "This repository has not published a release yet."
	}
	var apiErr *ghapi.Error
	if errors.As(err, &apiErr) && apiErr.Message != "" {
		return apiErr.Message
	}
	return err.Error()
}

// CheckForUpdates compares the installed tag against the repo's current latest
// release, and installs it when auto-update is on (PRD §10).
func (s *Service) CheckForUpdates(owner, name string) (App, error) {
	if _, ok := s.store.App(owner, name); !ok {
		return App{}, fmt.Errorf("%s/%s is not installed", owner, name)
	}

	s.forgetRelease(owner, name)
	release, err := s.latestRelease(s.context(), owner, name)
	if err != nil {
		if errors.Is(err, ghapi.ErrNoReleases) {
			_, _ = s.store.MutateApp(owner, name, func(a *store.App) { a.LastChecked = time.Now() })
			updated, _ := s.store.App(owner, name)
			return s.appView(updated, false), nil
		}
		return App{}, err
	}

	if _, err := s.store.MutateApp(owner, name, func(a *store.App) {
		a.LatestVersion = release.TagName
		a.LastChecked = time.Now()
	}); err != nil {
		return App{}, err
	}

	updated, _ := s.store.App(owner, name)
	if updated.AutoUpdate && updated.HasUpdate() {
		view, err := s.Install(owner, name, "")
		if err != nil {
			s.emit(EventToast, Toast{Kind: "error", Message: fmt.Sprintf("Auto-update of %s failed: %s", name, err)})
			return s.appView(updated, false), nil
		}
		s.emit(EventToast, Toast{Kind: "success", Message: fmt.Sprintf("%s updated to %s.", name, view.Version)})
		return view, nil
	}

	view := s.appView(updated, false)
	s.emit(EventApps, s.Installed())
	return view, nil
}

// CheckAllForUpdates refreshes every installed app, which is the Installed
// view's "Check all for updates" action (PRD §10).
func (s *Service) CheckAllForUpdates() ([]App, error) {
	apps := s.store.Apps()

	sem := make(chan struct{}, discoverConcurrency)
	var wg sync.WaitGroup
	var firstErr error
	var mu sync.Mutex

	for _, a := range apps {
		wg.Add(1)
		go func(a store.App) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if _, err := s.CheckForUpdates(a.Owner, a.Repo); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("%s: %w", a.Repo, err)
				}
				mu.Unlock()
			}
		}(a)
	}
	wg.Wait()

	out := s.Installed()
	s.emit(EventApps, out)
	return out, firstErr
}

// SetAutoUpdate toggles per-app auto-update (PRD §10). Enabling it runs a check
// immediately, so the app lands on the latest release without another click.
func (s *Service) SetAutoUpdate(owner, name string, enabled bool) (App, error) {
	ok, err := s.store.MutateApp(owner, name, func(a *store.App) { a.AutoUpdate = enabled })
	if err != nil {
		return App{}, err
	}
	if !ok {
		return App{}, fmt.Errorf("%s/%s is not installed", owner, name)
	}
	if enabled {
		return s.CheckForUpdates(owner, name)
	}
	updated, _ := s.store.App(owner, name)
	view := s.appView(updated, false)
	s.emit(EventApps, s.Installed())
	return view, nil
}

// Uninstall deletes the binary and forgets the app, so the repo reverts to
// "Not installed" in Discover (PRD §10).
func (s *Service) Uninstall(owner, name string) error {
	if err := s.install.RemoveBinary(name); err != nil {
		return err
	}
	if err := s.store.DeleteApp(owner, name); err != nil {
		return err
	}
	s.forgetRelease(owner, name)
	s.emit(EventApps, s.Installed())
	return nil
}

// AssetPattern is the filename contract for a given repo on this platform,
// shown in the Incompatible banner (PRD §9).
func (s *Service) AssetPattern(repo string) string {
	if repo == "" {
		repo = "{repo}"
	}
	return assetmatch.Pattern(repo, runtime.GOOS)
}
