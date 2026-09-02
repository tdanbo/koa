package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/playdead/koa/internal/ghapi"
	"github.com/playdead/koa/internal/installer"
)

// selfUpdateCheckInterval is how often koa re-checks its own latest release
// while it stays open. The first check happens shortly after startup.
const selfUpdateCheckInterval = 6 * time.Hour

// SelfUpdateInfo is what the frontend shows when a newer koa release exists.
type SelfUpdateInfo struct {
	Available   bool   `json:"available"`
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	PublishedAt string `json:"publishedAt"`
	URL         string `json:"url"`
}

// SelfUpdateProgress mirrors InstallProgress for koa's own update, streamed
// over EventSelfUpdate so the UI can show download/extract/install progress.
type SelfUpdateProgress struct {
	Stage string `json:"stage"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
	Error string `json:"error"`
}

// CheckSelfUpdate is the frontend-bound entry point for checking koa's own
// latest release.
func (s *Service) CheckSelfUpdate() (SelfUpdateInfo, error) {
	return s.checkSelfUpdate(s.context())
}

// checkSelfUpdate compares the running build against koa's own latest GitHub
// release. A dev build (no embedded version, or no configured self-update
// repo) never reports an update, since there is nothing meaningful to
// replace it with.
func (s *Service) checkSelfUpdate(ctx context.Context) (SelfUpdateInfo, error) {
	info := SelfUpdateInfo{Current: s.version}
	if s.selfUpdateOwner == "" || s.selfUpdateName == "" || s.version == "" || s.version == "dev" {
		s.setSelfUpdateInfo(info)
		return info, nil
	}

	release, err := s.gh.LatestRelease(ctx, s.selfUpdateOwner, s.selfUpdateName)
	if err != nil {
		if errors.Is(err, ghapi.ErrNoReleases) {
			s.setSelfUpdateInfo(info)
			return info, nil
		}
		return SelfUpdateInfo{}, err
	}

	info.Latest = release.TagName
	info.PublishedAt = release.PublishedAt.Format(time.RFC3339)
	info.URL = release.HTMLURL
	info.Available = release.TagName != "" && release.TagName != s.version

	s.setSelfUpdateInfo(info)
	if info.Available {
		s.emit(EventSelfUpdate, SelfUpdateProgress{Stage: "available"})
	}
	return info, nil
}

// SelfUpdateStatus returns the last-known result of CheckSelfUpdate without
// making a network call, so the frontend can render state instantly.
func (s *Service) SelfUpdateStatus() SelfUpdateInfo {
	s.selfUpdateMu.RLock()
	defer s.selfUpdateMu.RUnlock()
	return s.selfUpdateInfo
}

func (s *Service) setSelfUpdateInfo(info SelfUpdateInfo) {
	s.selfUpdateMu.Lock()
	s.selfUpdateInfo = info
	s.selfUpdateMu.Unlock()
}

// watchSelfUpdate checks once shortly after startup, then on a slow interval
// for as long as koa stays open — the UI only needs to notice eventually.
func (s *Service) watchSelfUpdate(ctx context.Context) {
	if s.selfUpdateOwner == "" || s.selfUpdateName == "" {
		return
	}
	if _, err := s.checkSelfUpdate(ctx); err != nil {
		return
	}
	ticker := time.NewTicker(selfUpdateCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = s.checkSelfUpdate(ctx)
		}
	}
}

// SelfUpdate is the frontend-bound entry point: downloads koa's latest
// release, replaces the running executable in place, and relaunches it.
func (s *Service) SelfUpdate() error {
	execPath, err := selfExecutablePath()
	if err != nil {
		s.emit(EventSelfUpdate, SelfUpdateProgress{Stage: "failed", Error: err.Error()})
		return err
	}
	return s.selfUpdateTo(s.context(), execPath)
}

// selfUpdateTo is SelfUpdate with the executable path passed in explicitly,
// which is what makes the swap-and-relaunch pipeline testable without
// touching the real running binary.
func (s *Service) selfUpdateTo(ctx context.Context, execPath string) error {
	if !s.selfUpdating.CompareAndSwap(false, true) {
		return errors.New("an update is already in progress")
	}
	defer s.selfUpdating.Store(false)

	emit := func(stage string, done, total int64, errText string) {
		s.emit(EventSelfUpdate, SelfUpdateProgress{Stage: stage, Done: done, Total: total, Error: errText})
	}

	result, err := s.install.InstallAt(ctx, installer.Request{
		Owner: s.selfUpdateOwner,
		Repo:  s.selfUpdateName,
	}, execPath, func(stage installer.Stage, done, total int64) {
		emit(string(stage), done, total, "")
	})
	if err != nil {
		msg := installErrorMessage(err)
		emit("failed", 0, 0, msg)
		return errors.New(msg)
	}

	s.setSelfUpdateInfo(SelfUpdateInfo{Current: result.Tag})
	emit("relaunching", 0, 0, "")

	if err := relaunch(execPath); err != nil {
		emit("failed", 0, 0, fmt.Sprintf("updated to %s but could not restart automatically — start Koa again: %v", result.Tag, err))
		return nil
	}

	go func() {
		time.Sleep(500 * time.Millisecond)
		s.hostMu.RLock()
		host := s.host
		s.hostMu.RUnlock()
		host.Quit()
	}()
	return nil
}

// selfExecutablePath resolves the real, symlink-free path of the running
// binary, which is what self-update must overwrite.
func selfExecutablePath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate the running Koa binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved, nil
	}
	return p, nil
}

// relaunch starts a fresh copy of the just-installed binary, detached from
// this process, so the user lands back in a running koa after the swap.
func relaunch(execPath string) error {
	cmd := exec.Command(execPath)
	cmd.Dir = filepath.Dir(execPath)
	return cmd.Start()
}

// sweepSelfUpdateLeftover removes a stale ".old" copy of koa's own executable
// left behind by a self-update on a platform that could not delete the
// previous binary while it was still running (PRD §10's Windows lock case,
// applied to koa's own exe rather than the koa bin folder).
func sweepSelfUpdateLeftover() {
	execPath, err := selfExecutablePath()
	if err != nil {
		return
	}
	_ = os.Remove(execPath + ".old")
}

// parseSelfUpdateRepo splits an "owner/repo" coordinate. An empty or
// malformed value disables self-update entirely rather than guessing.
func parseSelfUpdateRepo(v string) (owner, repo string) {
	owner, repo, ok := strings.Cut(strings.TrimSpace(v), "/")
	if !ok || owner == "" || repo == "" {
		return "", ""
	}
	return owner, repo
}
