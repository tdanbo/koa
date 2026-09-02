package installer

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/playdead/koa/internal/assetmatch"
)

// maxBinarySize caps how much koa will write out of an archive, so a hostile
// or broken asset cannot fill the disk.
const maxBinarySize = 512 << 20 // 512 MiB

// errBinaryNotFound means the archive extracted cleanly but held nothing that
// looked like the repo's binary.
var errBinaryNotFound = errors.New("no binary found in release asset")

// sidecarExtensions are documentation and metadata files that ship alongside a
// binary and must never be mistaken for it (PRD §9).
var sidecarExtensions = map[string]bool{
	".md": true, ".txt": true, ".json": true, ".yaml": true, ".yml": true,
	".toml": true, ".sha256": true, ".sig": true, ".asc": true, ".pdf": true,
	".html": true, ".png": true, ".svg": true,
}

// sidecarNames are extension-less files that are documentation by convention.
var sidecarNames = map[string]bool{
	"license": true, "licence": true, "readme": true, "changelog": true,
	"notice": true, "authors": true, "copying": true, "version": true,
}

// isSidecar reports whether an archive entry is an accompanying file rather
// than a candidate binary.
func isSidecar(name string) bool {
	base := strings.ToLower(path.Base(name))
	if sidecarNames[base] {
		return true
	}
	ext := filepath.Ext(base)
	if sidecarExtensions[ext] {
		return true
	}
	if stem := strings.TrimSuffix(base, ext); sidecarNames[stem] && ext != ".exe" {
		return true
	}
	return false
}

// binaryCandidate scores an archive entry against the expected command name.
// A higher score wins; 0 means "not a candidate".
func binaryCandidate(entry, repo, goos string) int {
	if isSidecar(entry) {
		return 0
	}
	base := strings.ToLower(path.Base(entry))
	want := strings.ToLower(repo)
	if goos == "windows" {
		want += ".exe"
	}
	switch {
	case base == want:
		return 3
	case base == strings.ToLower(repo), base == strings.ToLower(repo)+".exe":
		return 2
	case goos == "windows" && strings.HasSuffix(base, ".exe"):
		return 1
	case goos != "windows" && filepath.Ext(base) == "":
		return 1
	}
	return 0
}

// extract pulls the repo's binary out of a downloaded asset and writes it to
// dest. It never writes archive-controlled paths — every entry is copied to
// the single fixed destination — so archive path traversal cannot apply.
func extract(archivePath, dest, repo, goos string, kind assetmatch.Kind) error {
	switch kind {
	case assetmatch.KindRaw:
		return copyFileTo(archivePath, dest)
	case assetmatch.KindTarGz:
		return extractTarGz(archivePath, dest, repo, goos)
	case assetmatch.KindZip:
		return extractZip(archivePath, dest, repo, goos)
	default:
		return fmt.Errorf("unsupported asset kind %q", kind)
	}
}

func extractTarGz(archivePath, dest, repo, goos string) error {
	// Two passes: find the best-scoring entry, then extract just that one.
	best, err := scanTarGz(archivePath, repo, goos)
	if err != nil {
		return err
	}
	if best == "" {
		return errBinaryNotFound
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open asset: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("read gzip asset: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar asset: %w", err)
		}
		if hdr.Name != best {
			continue
		}
		return writeBinary(dest, tr)
	}
	return errBinaryNotFound
}

func scanTarGz(archivePath, repo, goos string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("open asset: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("read gzip asset: %w", err)
	}
	defer gz.Close()

	var bestName string
	bestScore := 0
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar asset: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if score := binaryCandidate(hdr.Name, repo, goos); score > bestScore {
			bestName, bestScore = hdr.Name, score
		}
	}
	return bestName, nil
}

func extractZip(archivePath, dest, repo, goos string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("read zip asset: %w", err)
	}
	defer zr.Close()

	var best *zip.File
	bestScore := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if score := binaryCandidate(f.Name, repo, goos); score > bestScore {
			best, bestScore = f, score
		}
	}
	if best == nil {
		return errBinaryNotFound
	}

	rc, err := best.Open()
	if err != nil {
		return fmt.Errorf("open %s in zip: %w", best.Name, err)
	}
	defer rc.Close()
	return writeBinary(dest, rc)
}

// writeBinary streams r into dest with the executable bit set.
func writeBinary(dest string, r io.Reader) error {
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	written, err := io.Copy(out, io.LimitReader(r, maxBinarySize+1))
	if err != nil {
		out.Close()
		return fmt.Errorf("write %s: %w", dest, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close %s: %w", dest, err)
	}
	if written > maxBinarySize {
		return fmt.Errorf("binary exceeds the %d MiB limit koa will install", maxBinarySize>>20)
	}
	if written == 0 {
		return errBinaryNotFound
	}
	return nil
}

func copyFileTo(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open asset: %w", err)
	}
	defer in.Close()
	return writeBinary(dest, in)
}
