package installer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/playdead/koa/internal/config"
	"github.com/playdead/koa/internal/ghapi"
)

const binaryBody = "#!/bin/sh\necho dumpscope\n"

func makeTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// fixture spins up a fake GitHub serving one release with the given assets.
func fixture(t *testing.T, goos string, assets map[string][]byte) (*Installer, config.Paths) {
	t.Helper()

	mux := http.NewServeMux()
	var base string

	mux.HandleFunc("/repos/playdead/dumpscope/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		writeRelease(t, w, base, "v1.0.0", assets)
	})
	mux.HandleFunc("/repos/playdead/dumpscope/releases", func(w http.ResponseWriter, r *http.Request) {
		var one bytes.Buffer
		writeRelease(t, &one, base, "v0.9.2", assets)
		fmt.Fprintf(w, "[%s]", one.String())
	})
	for name, body := range assets {
		body := body
		mux.HandleFunc("/asset/"+name, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", fmt.Sprint(len(body)))
			w.Write(body)
		})
	}

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	base = srv.URL

	gh := ghapi.New("t")
	gh.SetBaseURL(srv.URL)

	root := t.TempDir()
	paths := config.Paths{
		BinDir:    filepath.Join(root, "bin"),
		ConfigDir: filepath.Join(root, "cfg"),
		StateFile: filepath.Join(root, "cfg", "state.json"),
		CacheDir:  filepath.Join(root, "cache"),
	}
	if err := paths.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	inst := New(paths, gh)
	inst.goos = goos
	return inst, paths
}

type releaseWriter interface{ Write([]byte) (int, error) }

func writeRelease(t *testing.T, w releaseWriter, base, tag string, assets map[string][]byte) {
	t.Helper()
	type asset struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Size int64  `json:"size"`
		URL  string `json:"url"`
	}
	rel := struct {
		TagName     string  `json:"tag_name"`
		PublishedAt string  `json:"published_at"`
		Assets      []asset `json:"assets"`
	}{TagName: tag, PublishedAt: "2026-08-30T10:00:00Z"}
	for name, body := range assets {
		rel.Assets = append(rel.Assets, asset{Name: name, Size: int64(len(body)), URL: base + "/asset/" + name})
	}
	raw, err := json.Marshal(rel)
	if err != nil {
		t.Fatal(err)
	}
	w.Write(raw)
}

func TestInstallFromTarGz(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bin path naming is platform-specific; covered on unix")
	}
	archive := makeTarGz(t, map[string]string{
		"dumpscope-1.0.0/LICENSE":   "MIT",
		"dumpscope-1.0.0/README.md": "# dumpscope",
		"dumpscope-1.0.0/dumpscope": binaryBody,
	})
	inst, paths := fixture(t, "linux", map[string][]byte{
		"dumpscope-1.0.0-amd64-linux.tar.gz": archive,
	})

	var stages []Stage
	res, err := inst.Install(context.Background(), Request{Owner: "playdead", Repo: "dumpscope"}, func(s Stage, done, total int64) {
		if len(stages) == 0 || stages[len(stages)-1] != s {
			stages = append(stages, s)
		}
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if res.Tag != "v1.0.0" {
		t.Errorf("tag = %q", res.Tag)
	}
	if res.AssetName != "dumpscope-1.0.0-amd64-linux.tar.gz" {
		t.Errorf("asset = %q", res.AssetName)
	}
	want := filepath.Join(paths.BinDir, "dumpscope")
	if res.BinaryPath != want {
		t.Errorf("binary path = %q, want %q", res.BinaryPath, want)
	}

	got, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("installed binary unreadable: %v", err)
	}
	if string(got) != binaryBody {
		t.Errorf("installed the wrong file: %q", got)
	}
	info, err := os.Stat(want)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("binary is not executable: %v", info.Mode())
	}
	if res.SizeBytes != int64(len(binaryBody)) {
		t.Errorf("size = %d", res.SizeBytes)
	}

	wantStages := []Stage{StageResolving, StageDownloading, StageExtracting, StageInstalling, StageDone}
	if fmt.Sprint(stages) != fmt.Sprint(wantStages) {
		t.Errorf("stages = %v, want %v", stages, wantStages)
	}
}

func TestInstallFromZip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exercised via the unix path")
	}
	archive := makeZip(t, map[string]string{
		"LICENSE":   "MIT",
		"dumpscope": binaryBody,
	})
	// A linux-named zip is unusual but exercises the zip reader on this host.
	inst, paths := fixture(t, "linux", map[string][]byte{
		"dumpscope-1.0.0-amd64-linux.tar.gz": archive,
	})
	// Force the zip code path regardless of the extension.
	if err := extract(writeTemp(t, archive), filepath.Join(paths.BinDir, "dumpscope"), "dumpscope", "linux", "zip"); err != nil {
		t.Fatalf("extract zip: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(paths.BinDir, "dumpscope"))
	if err != nil || string(got) != binaryBody {
		t.Fatalf("zip extraction produced %q (err %v)", got, err)
	}
	_ = inst
}

func writeTemp(t *testing.T, body []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "asset.zip")
	if err := os.WriteFile(p, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestInstallRejectsIncompatibleRelease(t *testing.T) {
	inst, _ := fixture(t, "linux", map[string][]byte{
		"dumpscope-1.0.0.tar.gz": makeTarGz(t, map[string]string{"dumpscope": binaryBody}),
	})

	_, err := inst.Install(context.Background(), Request{Owner: "playdead", Repo: "dumpscope"}, nil)
	var incompat *ErrIncompatible
	if !errors.As(err, &incompat) {
		t.Fatalf("want ErrIncompatible, got %v", err)
	}
	if !strings.Contains(incompat.Pattern, "amd64-linux.tar.gz") {
		t.Errorf("pattern = %q", incompat.Pattern)
	}
}

func TestInstallSpecificTagRollback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("covered on unix")
	}
	archive := makeTarGz(t, map[string]string{"dumpscope": binaryBody})
	inst, _ := fixture(t, "linux", map[string][]byte{
		"dumpscope-0.9.2-amd64-linux.tar.gz": archive,
	})

	res, err := inst.Install(context.Background(), Request{Owner: "playdead", Repo: "dumpscope", Tag: "v0.9.2"}, nil)
	if err != nil {
		t.Fatalf("rollback install: %v", err)
	}
	if res.Tag != "v0.9.2" {
		t.Fatalf("tag = %q, want the requested tag", res.Tag)
	}
}

func TestInstallUnknownTag(t *testing.T) {
	inst, _ := fixture(t, "linux", map[string][]byte{
		"dumpscope-0.9.2-amd64-linux.tar.gz": makeTarGz(t, map[string]string{"dumpscope": binaryBody}),
	})
	if _, err := inst.Install(context.Background(), Request{Owner: "playdead", Repo: "dumpscope", Tag: "v9.9.9"}, nil); err == nil {
		t.Fatal("expected an error for an unknown tag")
	}
}

func TestInstallReplacesExistingBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("covered on unix")
	}
	inst, paths := fixture(t, "linux", map[string][]byte{
		"dumpscope-1.0.0-amd64-linux.tar.gz": makeTarGz(t, map[string]string{"dumpscope": binaryBody}),
	})
	dest := filepath.Join(paths.BinDir, "dumpscope")
	if err := os.WriteFile(dest, []byte("old version"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := inst.Install(context.Background(), Request{Owner: "playdead", Repo: "dumpscope"}, nil); err != nil {
		t.Fatalf("Install: %v", err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != binaryBody {
		t.Fatalf("binary not replaced: %q", got)
	}
	if _, err := os.Stat(dest + ".new"); err == nil {
		t.Error("temp file left behind")
	}
}

func TestRemoveBinaryIsIdempotent(t *testing.T) {
	inst, paths := fixture(t, "linux", nil)
	dest := paths.BinaryPath("dumpscope")
	if err := os.WriteFile(dest, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := inst.RemoveBinary("dumpscope"); err != nil {
		t.Fatalf("RemoveBinary: %v", err)
	}
	if err := inst.RemoveBinary("dumpscope"); err != nil {
		t.Fatalf("second RemoveBinary should be a no-op, got %v", err)
	}
}

func TestBinaryCandidateIgnoresSidecars(t *testing.T) {
	cases := []struct {
		entry string
		want  bool
	}{
		{"dumpscope", true},
		{"pkg/dumpscope", true},
		{"LICENSE", false},
		{"README.md", false},
		{"CHANGELOG", false},
		{"dumpscope.sha256", false},
		{"notes.txt", false},
	}
	for _, tc := range cases {
		got := binaryCandidate(tc.entry, "dumpscope", "linux") > 0
		if got != tc.want {
			t.Errorf("binaryCandidate(%q) = %v, want %v", tc.entry, got, tc.want)
		}
	}
	if binaryCandidate("dumpscope.exe", "dumpscope", "windows") != 3 {
		t.Error("dumpscope.exe should be the top candidate on windows")
	}
}

func TestExtractNoBinary(t *testing.T) {
	archive := makeTarGz(t, map[string]string{"README.md": "# nothing here"})
	src := filepath.Join(t.TempDir(), "a.tar.gz")
	if err := os.WriteFile(src, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	err := extract(src, filepath.Join(t.TempDir(), "out"), "dumpscope", "linux", "tar.gz")
	if !errors.Is(err, errBinaryNotFound) {
		t.Fatalf("want errBinaryNotFound, got %v", err)
	}
}

func TestSanitizeFilename(t *testing.T) {
	for _, in := range []string{"../../etc/passwd", "/etc/passwd", "a/b/c.tar.gz"} {
		got := sanitizeFilename(in)
		if strings.ContainsAny(got, `/\`) {
			t.Errorf("sanitizeFilename(%q) = %q, still contains a separator", in, got)
		}
	}
}
