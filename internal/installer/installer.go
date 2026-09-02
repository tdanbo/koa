// Package installer performs the install, update and rollback steps of PRD §10:
// match a release asset against the naming convention, download it, extract the
// binary, and place it in the koa bin folder under a clean command name.
package installer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/playdead/koa/internal/assetmatch"
	"github.com/playdead/koa/internal/config"
	"github.com/playdead/koa/internal/ghapi"
)

// Stage names the phase of an install, for the progress UI (PRD §5.5).
type Stage string

const (
	StageResolving   Stage = "resolving"
	StageDownloading Stage = "downloading"
	StageExtracting  Stage = "extracting"
	StageInstalling  Stage = "installing"
	StageDone        Stage = "done"
)

// Progress is called as an install advances. Done and Total are byte counts
// during StageDownloading and zero otherwise.
type Progress func(stage Stage, done, total int64)

// ErrIncompatible means no asset in the target release matches the naming
// convention for this platform, so the repo cannot be installed (PRD §9).
type ErrIncompatible struct {
	Repo    string
	Tag     string
	Pattern string
}

func (e *ErrIncompatible) Error() string {
	return fmt.Sprintf("no asset in %s matches %s", e.Tag, e.Pattern)
}

// Installer owns the file-system side of installing binaries.
type Installer struct {
	paths config.Paths
	gh    *ghapi.Client
	goos  string
}

// New returns an Installer writing into paths.
func New(paths config.Paths, gh *ghapi.Client) *Installer {
	return &Installer{paths: paths, gh: gh, goos: runtime.GOOS}
}

// Request describes what to install. An empty Tag means "the latest release",
// which is what a fresh install and an update both use; a specific tag is how
// rollback and reinstall work (PRD §10).
type Request struct {
	Owner string
	Repo  string
	Tag   string
}

// Result reports what ended up on disk.
type Result struct {
	Tag         string
	AssetName   string
	BinaryPath  string
	SizeBytes   int64
	PublishedAt time.Time
}

// Install runs the whole pipeline for one request, writing the binary into
// the koa bin folder under its clean command name.
func (i *Installer) Install(ctx context.Context, req Request, onProgress Progress) (Result, error) {
	return i.installTo(ctx, req, i.paths.BinaryPath(req.Repo), onProgress)
}

// InstallAt behaves like Install but writes the binary to dest instead of the
// koa bin folder. This is how koa updates its own running executable, which
// may live anywhere on disk depending on how it was originally installed.
func (i *Installer) InstallAt(ctx context.Context, req Request, dest string, onProgress Progress) (Result, error) {
	return i.installTo(ctx, req, dest, onProgress)
}

func (i *Installer) installTo(ctx context.Context, req Request, dest string, onProgress Progress) (Result, error) {
	report := func(stage Stage, done, total int64) {
		if onProgress != nil {
			onProgress(stage, done, total)
		}
	}

	report(StageResolving, 0, 0)
	release, err := i.resolveRelease(ctx, req)
	if err != nil {
		return Result{}, err
	}

	match, ok := assetmatch.PickFor(req.Repo, release.AssetNames(), i.goos)
	if !ok {
		return Result{}, &ErrIncompatible{
			Repo:    req.Repo,
			Tag:     release.TagName,
			Pattern: assetmatch.Pattern(req.Repo, i.goos),
		}
	}
	asset, ok := release.AssetByName(match.Filename)
	if !ok {
		return Result{}, fmt.Errorf("release %s no longer lists asset %s", release.TagName, match.Filename)
	}

	if err := i.paths.EnsureDirs(); err != nil {
		return Result{}, err
	}

	workDir, err := os.MkdirTemp(i.paths.CacheDir, "install-*")
	if err != nil {
		return Result{}, fmt.Errorf("create work directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	report(StageDownloading, 0, asset.Size)
	archivePath := filepath.Join(workDir, sanitizeFilename(asset.Name))
	if err := i.download(ctx, asset, archivePath, func(done, total int64) {
		report(StageDownloading, done, total)
	}); err != nil {
		return Result{}, err
	}

	report(StageExtracting, 0, 0)
	staged := filepath.Join(workDir, config.CommandName(req.Repo))
	if err := extract(archivePath, staged, req.Repo, i.goos, match.Kind); err != nil {
		if errors.Is(err, errBinaryNotFound) {
			return Result{}, fmt.Errorf("%s contains no binary named %s", asset.Name, config.CommandName(req.Repo))
		}
		return Result{}, err
	}

	report(StageInstalling, 0, 0)
	if err := placeBinary(staged, dest); err != nil {
		return Result{}, err
	}

	size := int64(0)
	if info, err := os.Stat(dest); err == nil {
		size = info.Size()
	}

	report(StageDone, size, size)
	return Result{
		Tag:         release.TagName,
		AssetName:   asset.Name,
		BinaryPath:  dest,
		SizeBytes:   size,
		PublishedAt: release.PublishedAt,
	}, nil
}

// resolveRelease turns a Request into the concrete release to install.
func (i *Installer) resolveRelease(ctx context.Context, req Request) (ghapi.Release, error) {
	if req.Tag == "" {
		return i.gh.LatestRelease(ctx, req.Owner, req.Repo)
	}
	// A specific tag: fetch the list and find it, so rollback works for tags
	// that are no longer "latest".
	releases, err := i.gh.ListReleases(ctx, req.Owner, req.Repo, 100)
	if err != nil {
		return ghapi.Release{}, err
	}
	for _, r := range releases {
		if r.TagName == req.Tag {
			return r, nil
		}
	}
	return ghapi.Release{}, fmt.Errorf("release %s not found in %s/%s", req.Tag, req.Owner, req.Repo)
}

func (i *Installer) download(ctx context.Context, asset ghapi.Asset, dest string, onProgress ghapi.Progress) error {
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create download file: %w", err)
	}
	if err := i.gh.DownloadAsset(ctx, asset, f, onProgress); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close download file: %w", err)
	}
	return nil
}

// RemoveBinary deletes an installed binary from the koa bin folder. A missing
// file is not an error — uninstall should still succeed and clear the record.
func (i *Installer) RemoveBinary(repo string) error {
	path := i.paths.BinaryPath(repo)
	err := os.Remove(path)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	// Windows refuses to delete a running executable; move it aside so the
	// bin folder is clean and let the stale copy be swept up later.
	if aside := path + ".removed"; os.Rename(path, aside) == nil {
		_ = os.Remove(aside)
		return nil
	}
	return fmt.Errorf("remove %s: %w", path, err)
}

// BinarySize reports the on-disk size of an installed binary, or 0 if absent.
func (i *Installer) BinarySize(repo string) int64 {
	info, err := os.Stat(i.paths.BinaryPath(repo))
	if err != nil {
		return 0
	}
	return info.Size()
}

// Sweep deletes leftovers from interrupted installs and Windows replacements.
func (i *Installer) Sweep() {
	entries, err := os.ReadDir(i.paths.BinDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) == ".removed" || filepath.Ext(name) == ".old" {
			_ = os.Remove(filepath.Join(i.paths.BinDir, name))
		}
	}
	if entries, err := os.ReadDir(i.paths.CacheDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), "install-") {
				_ = os.RemoveAll(filepath.Join(i.paths.CacheDir, e.Name()))
			}
		}
	}
}

// placeBinary moves the staged binary into its final location, replacing any
// previous version. The write goes to a sibling temp path first so a failure
// never leaves a half-written command on PATH.
func placeBinary(staged, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create bin folder: %w", err)
	}

	tmp := dest + ".new"
	_ = os.Remove(tmp)
	if err := copyFileTo(staged, tmp); err != nil {
		return err
	}
	if err := platformFinalizePermissions(tmp); err != nil {
		return fmt.Errorf("chmod %s: %w", tmp, err)
	}

	if err := os.Rename(tmp, dest); err == nil {
		return nil
	}

	// Windows holds a lock on a running .exe: move the old one aside, then
	// retry. The displaced file is cleaned up by Sweep on a later run.
	aside := dest + ".old"
	_ = os.Remove(aside)
	if err := os.Rename(dest, aside); err != nil && !errors.Is(err, fs.ErrNotExist) {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace %s (is it still running?): %w", dest, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Rename(aside, dest)
		_ = os.Remove(tmp)
		return fmt.Errorf("install %s: %w", dest, err)
	}
	_ = os.Remove(aside)
	return nil
}

// sanitizeFilename strips any directory component from an asset name so a
// crafted name cannot escape the work directory.
func sanitizeFilename(name string) string {
	base := filepath.Base(filepath.FromSlash(name))
	if base == "." || base == string(filepath.Separator) || base == ".." {
		return "asset"
	}
	return base
}
