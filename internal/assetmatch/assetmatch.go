// Package assetmatch implements koa's release-asset naming convention (PRD §9).
//
// The contract maintainers follow is:
//
//	{repo-name}-{version}-{arch}-{os}{ext}
//
//	myapp-1.2.0-amd64-linux.tar.gz
//	myapp-1.2.0-amd64-windows.zip
//
// An asset qualifies when its filename starts with `{repo-name}-`, contains a
// recognised amd64 arch keyword, and ends with the OS keyword plus a supported
// extension for the running platform. The version segment in the middle is
// never parsed for meaning — whatever is there, is there.
package assetmatch

import (
	"fmt"
	"runtime"
	"strings"
)

// Kind describes how a matched asset is packaged.
type Kind string

const (
	// KindTarGz is a gzipped tar archive containing the binary.
	KindTarGz Kind = "tar.gz"
	// KindZip is a zip archive containing the binary.
	KindZip Kind = "zip"
	// KindRaw is a bare binary published without an archive.
	KindRaw Kind = "raw"
)

// archKeywords are the amd64 spellings koa accepts. `amd64` is canonical and
// the one documented for maintainers; the others are common equivalents.
var archKeywords = []string{"amd64", "x86_64", "x64"}

// platformSuffix pairs an OS keyword with the extensions valid for it.
type platformSuffix struct {
	os   string
	ext  string
	kind Kind
}

var suffixesByOS = map[string][]platformSuffix{
	"linux": {
		{os: "linux", ext: ".tar.gz", kind: KindTarGz},
		{os: "linux", ext: ".tgz", kind: KindTarGz},
		{os: "linux", ext: "", kind: KindRaw},
	},
	"windows": {
		{os: "windows", ext: ".zip", kind: KindZip},
		{os: "windows", ext: ".exe", kind: KindRaw},
	},
}

// Result describes a successful match.
type Result struct {
	// Filename is the asset filename exactly as published.
	Filename string
	// Version is the segment between the repo name and the arch keyword. It is
	// informational only — koa never reconstructs or compares it.
	Version string
	// Arch is the arch keyword that matched.
	Arch string
	// OS is the OS keyword that matched.
	OS string
	// Kind is how the asset is packaged.
	Kind Kind
}

// Pattern returns the human-readable filename pattern koa expects for the
// given repo on the given OS. It is what the Incompatible banner names (§5.3).
func Pattern(repo, goos string) string {
	switch goos {
	case "windows":
		return repo + "-{version}-amd64-windows.zip"
	default:
		return repo + "-{version}-amd64-linux.tar.gz"
	}
}

// Match reports whether filename is a koa-installable asset for repo on the
// current platform.
func Match(repo, filename string) (Result, bool) {
	return MatchFor(repo, filename, runtime.GOOS)
}

// MatchFor is Match against an explicit GOOS, so the rule can be exercised for
// either platform regardless of where koa is running.
func MatchFor(repo, filename, goos string) (Result, bool) {
	suffixes, ok := suffixesByOS[goos]
	if !ok || repo == "" || filename == "" {
		return Result{}, false
	}

	lower := strings.ToLower(filename)
	prefix := strings.ToLower(repo) + "-"
	if !strings.HasPrefix(lower, prefix) {
		return Result{}, false
	}

	for _, sfx := range suffixes {
		tail := "-" + sfx.os + sfx.ext
		if !strings.HasSuffix(lower, tail) {
			continue
		}
		// The prefix and suffix can overlap on a degenerate name such as
		// `x-linux` for repo `x`, where both match the same bytes. There is no
		// middle to read in that case, so it cannot satisfy the convention.
		if len(prefix)+len(tail) > len(lower) {
			continue
		}
		// The middle is everything between "{repo}-" and "-{os}{ext}".
		middle := lower[len(prefix) : len(lower)-len(tail)]
		for _, arch := range archKeywords {
			archTail := "-" + arch
			if !strings.HasSuffix(middle, archTail) {
				continue
			}
			version := strings.TrimSuffix(middle, archTail)
			if version == "" {
				// `myapp-amd64-linux.tar.gz` has no version segment; the
				// convention requires one.
				continue
			}
			return Result{
				Filename: filename,
				Version:  versionSegment(filename, lower, len(prefix), len(version)),
				Arch:     arch,
				OS:       sfx.os,
				Kind:     sfx.kind,
			}, true
		}
	}
	return Result{}, false
}

// versionSegment reads the version out of the original filename so its case is
// preserved. Lower-casing is byte-length-preserving for the ASCII that asset
// names use in practice; if some name makes it differ, the lower-cased segment
// is returned rather than slicing the original out of bounds.
func versionSegment(filename, lower string, start, length int) string {
	if len(lower) == len(filename) && start+length <= len(filename) {
		return filename[start : start+length]
	}
	return lower[start : start+length]
}

// Pick returns the first asset in names that matches, preferring archives over
// raw binaries so that a release publishing both lands on the archive.
func Pick(repo string, names []string) (Result, bool) {
	return PickFor(repo, names, runtime.GOOS)
}

// PickFor is Pick against an explicit GOOS.
func PickFor(repo string, names []string, goos string) (Result, bool) {
	var raw Result
	var haveRaw bool
	for _, n := range names {
		res, ok := MatchFor(repo, n, goos)
		if !ok {
			continue
		}
		if res.Kind == KindRaw {
			if !haveRaw {
				raw, haveRaw = res, true
			}
			continue
		}
		return res, true
	}
	if haveRaw {
		return raw, true
	}
	return Result{}, false
}

// IncompatibleReason is the sentence shown when nothing matched (§5.3, §9).
func IncompatibleReason(repo, goos, tag string) string {
	if tag == "" {
		return fmt.Sprintf("No release asset matches %s.", Pattern(repo, goos))
	}
	return fmt.Sprintf("No release asset matches %s. Latest release %s publishes no compatible binary.", Pattern(repo, goos), tag)
}
