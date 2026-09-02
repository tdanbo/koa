package app

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/playdead/koa/internal/assetmatch"
	"github.com/playdead/koa/internal/ghapi"
	"github.com/playdead/koa/internal/store"
)

// releaseTTL is how long a fetched latest-release is reused before koa asks
// GitHub again. Discovery touches one endpoint per repo, so a short cache keeps
// navigation snappy without hiding real updates.
const releaseTTL = 5 * time.Minute

// discoverConcurrency bounds the parallel release lookups.
const discoverConcurrency = 6

type cachedRelease struct {
	release ghapi.Release
	err     error
	at      time.Time
}

// Discover returns every koa-tagged repo across the user's account and orgs
// (PRD §8). Passing refresh=false serves a cached result when one is fresh.
func (s *Service) Discover(refresh bool) (Discovery, error) {
	account := s.Account()
	if !account.SignedIn {
		return Discovery{Repos: []Repo{}, Scopes: []string{}, Errors: []ScopeError{}, SignedIn: false}, nil
	}

	if !refresh {
		s.discoverMu.Lock()
		cached := s.discoverCache
		s.discoverMu.Unlock()
		if cached != nil {
			// Install state can change without a re-search, so recompute it.
			fresh := *cached
			fresh.Repos = s.decorate(cached.Repos)
			return fresh, nil
		}
	}

	ctx := s.context()
	scopes, scopeErrs := s.searchScopes(ctx, account)

	type scopeResult struct {
		repos []ghapi.Repo
		err   ScopeError
		ok    bool
	}

	results := make([]scopeResult, len(scopes))
	var wg sync.WaitGroup
	for i, scope := range scopes {
		wg.Add(1)
		go func(i int, scope string) {
			defer wg.Done()
			repos, err := s.gh.SearchTopic(ctx, Topic, scope)
			if err != nil {
				results[i] = scopeResult{err: toScopeError(scope, err)}
				return
			}
			results[i] = scopeResult{repos: repos, ok: true}
		}(i, scope)
	}
	wg.Wait()

	seen := map[string]bool{}
	var merged []ghapi.Repo
	for _, r := range results {
		if !r.ok {
			scopeErrs = append(scopeErrs, r.err)
			continue
		}
		for _, repo := range r.repos {
			key := strings.ToLower(repo.FullName)
			if seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, repo)
		}
	}

	views := s.buildRepoViews(ctx, merged, account)

	out := Discovery{
		Repos:       views,
		Scopes:      scopes,
		Errors:      scopeErrs,
		RefreshedAt: time.Now(),
		SignedIn:    true,
	}
	if out.Errors == nil {
		out.Errors = []ScopeError{}
	}

	s.discoverMu.Lock()
	cache := out
	s.discoverCache = &cache
	s.discoverMu.Unlock()

	return out, nil
}

// invalidateDiscovery drops the cached Discover result, so the next visit
// re-queries GitHub. Called on sign-in and sign-out.
func (s *Service) invalidateDiscovery() {
	s.discoverMu.Lock()
	s.discoverCache = nil
	s.discoverMu.Unlock()

	s.releaseMu.Lock()
	s.releaseCache = map[string]cachedRelease{}
	s.releaseMu.Unlock()
}

// searchScopes builds the list of qualifiers to search: the user's own account
// plus every org they belong to, and any orgs typed by hand on the manual
// token path (PRD §7, §8).
func (s *Service) searchScopes(ctx context.Context, account Account) ([]string, []ScopeError) {
	scopes := []string{"user:" + account.Login}
	var errs []ScopeError

	orgs, err := s.gh.Orgs(ctx)
	if err != nil {
		errs = append(errs, toScopeError("your organizations", err))
	}

	seen := map[string]bool{strings.ToLower(account.Login): true}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[strings.ToLower(name)] {
			return
		}
		seen[strings.ToLower(name)] = true
		scopes = append(scopes, "org:"+name)
	}
	for _, o := range orgs {
		add(o.Login)
	}
	for _, o := range s.store.Settings().ManualOrgs {
		add(o)
	}
	return scopes, errs
}

// toScopeError turns an API failure into a per-scope banner, keeping the SSO
// case distinct so the user is told to authorize rather than shown "0 repos"
// (PRD §7).
func toScopeError(scope string, err error) ScopeError {
	out := ScopeError{Scope: scope, Message: err.Error()}
	var apiErr *ghapi.Error
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.SSO:
			out.SSO = true
			out.SSOURL = apiErr.SSOURL
			out.Message = "This organization enforces SAML SSO. Authorize your token for it on github.com, then refresh."
		case apiErr.RateLimited:
			out.Message = "GitHub rate limit reached. Try again shortly."
			if !apiErr.ResetAt.IsZero() {
				out.Message = fmt.Sprintf("GitHub rate limit reached. Try again after %s.", apiErr.ResetAt.Format("15:04"))
			}
		case apiErr.Message != "":
			out.Message = apiErr.Message
		}
	}
	return out
}

// buildRepoViews resolves each repo's latest release and install status.
func (s *Service) buildRepoViews(ctx context.Context, repos []ghapi.Repo, account Account) []Repo {
	views := make([]Repo, len(repos))

	sem := make(chan struct{}, discoverConcurrency)
	var wg sync.WaitGroup
	for i, repo := range repos {
		wg.Add(1)
		go func(i int, repo ghapi.Repo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			views[i] = s.repoView(ctx, repo, account)
		}(i, repo)
	}
	wg.Wait()

	sort.Slice(views, func(i, j int) bool {
		if strings.EqualFold(views[i].Name, views[j].Name) {
			return strings.ToLower(views[i].Owner) < strings.ToLower(views[j].Owner)
		}
		return strings.ToLower(views[i].Name) < strings.ToLower(views[j].Name)
	})
	if views == nil {
		return []Repo{}
	}
	return views
}

// repoView computes one Discover row.
func (s *Service) repoView(ctx context.Context, repo ghapi.Repo, account Account) Repo {
	owner := repo.Owner.Login
	view := Repo{
		ID:          store.Key(owner, repo.Name),
		Owner:       owner,
		Name:        repo.Name,
		Description: repo.Description,
		Visibility:  repo.Visibility(),
		HTMLURL:     repo.HTMLURL,
		OwnerScope:  ownerScope(owner, account.Login),
	}

	installed, isInstalled := s.store.App(owner, repo.Name)
	if isInstalled {
		view.Installed = true
		view.InstalledVersion = installed.Version
		view.AssetName = installed.AssetName
	}

	release, err := s.latestRelease(ctx, owner, repo.Name)
	switch {
	case errors.Is(err, ghapi.ErrNoReleases):
		view.Status = "No releases"
		view.StatusKind = StatusNoRelease
		view.IncompatibleReason = "This repository carries the koa topic but has not published a release yet."
		if isInstalled {
			applyInstalledStatus(&view, installed)
		}
		return view
	case err != nil:
		view.Status = "Unavailable"
		view.StatusKind = StatusNoRelease
		view.IncompatibleReason = err.Error()
		if isInstalled {
			applyInstalledStatus(&view, installed)
		}
		return view
	}

	view.LatestVersion = release.TagName
	view.PublishedAt = release.PublishedAt

	match, ok := assetmatch.Pick(repo.Name, release.AssetNames())
	if ok {
		view.AssetName = match.Filename
		if asset, found := release.AssetByName(match.Filename); found {
			view.AssetSize = asset.Size
		}
	}

	switch {
	case isInstalled:
		applyInstalledStatus(&view, installed)
		if !ok {
			// Installed but the newest release no longer ships a usable asset.
			view.Action = "Open"
			view.Status = "Installed " + installed.Version
			view.StatusKind = StatusInstalled
		}
	case !ok:
		view.Incompatible = true
		view.Status = "Incompatible"
		view.StatusKind = StatusIncompatible
		view.CanInstall = false
		view.IncompatibleReason = assetmatch.IncompatibleReason(repo.Name, runtime.GOOS, release.TagName)
	default:
		view.Status = "Not installed"
		view.StatusKind = StatusNotInstalled
		view.Action = "Install"
		view.CanInstall = true
	}

	return view
}

// applyInstalledStatus sets the status text for a repo koa already tracks.
func applyInstalledStatus(view *Repo, installed store.App) {
	view.Installed = true
	view.InstalledVersion = installed.Version
	view.CanInstall = true
	if view.LatestVersion != "" && view.LatestVersion != installed.Version {
		view.Status = "Update available"
		view.StatusKind = StatusUpdate
		view.Action = "Update"
		return
	}
	view.Status = "Installed " + installed.Version
	view.StatusKind = StatusInstalled
	view.Action = "Open"
}

// decorate recomputes install status over a cached repo list, so returning to
// Discover after installing something shows the new state without a refetch.
func (s *Service) decorate(repos []Repo) []Repo {
	out := make([]Repo, len(repos))
	copy(out, repos)
	for i := range out {
		installed, ok := s.store.App(out[i].Owner, out[i].Name)
		if !ok {
			if out[i].Installed {
				// Uninstalled since the cache was built.
				out[i].Installed = false
				out[i].InstalledVersion = ""
				if out[i].Incompatible {
					out[i].Status, out[i].StatusKind, out[i].Action, out[i].CanInstall = "Incompatible", StatusIncompatible, "", false
				} else {
					out[i].Status, out[i].StatusKind, out[i].Action, out[i].CanInstall = "Not installed", StatusNotInstalled, "Install", true
				}
			}
			continue
		}
		applyInstalledStatus(&out[i], installed)
	}
	return out
}

func ownerScope(owner, login string) string {
	if strings.EqualFold(owner, login) {
		return "you"
	}
	return owner
}

// latestRelease fetches and caches a repo's latest release.
func (s *Service) latestRelease(ctx context.Context, owner, repo string) (ghapi.Release, error) {
	key := store.Key(owner, repo)

	s.releaseMu.Lock()
	if s.releaseCache == nil {
		s.releaseCache = map[string]cachedRelease{}
	}
	if hit, ok := s.releaseCache[key]; ok && time.Since(hit.at) < releaseTTL {
		s.releaseMu.Unlock()
		return hit.release, hit.err
	}
	s.releaseMu.Unlock()

	release, err := s.gh.LatestRelease(ctx, owner, repo)

	s.releaseMu.Lock()
	s.releaseCache[key] = cachedRelease{release: release, err: err, at: time.Now()}
	s.releaseMu.Unlock()

	return release, err
}

// forgetRelease drops one repo's cached release, so an action that changes it
// (an install, an explicit update check) is reflected immediately.
func (s *Service) forgetRelease(owner, repo string) {
	s.releaseMu.Lock()
	delete(s.releaseCache, store.Key(owner, repo))
	s.releaseMu.Unlock()
}
