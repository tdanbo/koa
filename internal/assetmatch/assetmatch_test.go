package assetmatch

import (
	"strings"
	"testing"
)

func TestMatchForLinux(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		want    bool
		version string
		kind    Kind
	}{
		{"canonical", "myapp-1.2.0-amd64-linux.tar.gz", true, "1.2.0", KindTarGz},
		{"v-prefixed version", "myapp-v1.2.0-amd64-linux.tar.gz", true, "v1.2.0", KindTarGz},
		{"dotted version", "myapp-2026.09.02-amd64-linux.tar.gz", true, "2026.09.02", KindTarGz},
		{"x86_64 alias", "myapp-1.2.0-x86_64-linux.tar.gz", true, "1.2.0", KindTarGz},
		{"tgz", "myapp-1.2.0-amd64-linux.tgz", true, "1.2.0", KindTarGz},
		{"raw binary", "myapp-1.2.0-amd64-linux", true, "1.2.0", KindRaw},
		{"uppercase", "MyApp-1.2.0-AMD64-Linux.tar.gz", true, "1.2.0", KindTarGz},
		{"wrong os", "myapp-1.2.0-amd64-windows.zip", false, "", ""},
		{"wrong arch", "myapp-1.2.0-arm64-linux.tar.gz", false, "", ""},
		{"wrong repo", "otherapp-1.2.0-amd64-linux.tar.gz", false, "", ""},
		{"source archive", "myapp-1.2.0.tar.gz", false, "", ""},
		{"missing version", "myapp-amd64-linux.tar.gz", false, "", ""},
		{"repo prefix but different repo", "myapp-extra-1.2.0-amd64-linux.tar.gz", true, "extra-1.2.0", KindTarGz},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := MatchFor("myapp", tc.file, "linux")
			if ok != tc.want {
				t.Fatalf("MatchFor(%q) ok = %v, want %v", tc.file, ok, tc.want)
			}
			if !ok {
				return
			}
			if got.Version != tc.version {
				t.Errorf("version = %q, want %q", got.Version, tc.version)
			}
			if got.Kind != tc.kind {
				t.Errorf("kind = %q, want %q", got.Kind, tc.kind)
			}
			if got.Filename != tc.file {
				t.Errorf("filename = %q, want %q", got.Filename, tc.file)
			}
		})
	}
}

func TestMatchForWindows(t *testing.T) {
	if got, ok := MatchFor("myapp", "myapp-1.2.0-amd64-windows.zip", "windows"); !ok || got.Kind != KindZip {
		t.Fatalf("zip asset should match on windows, got %+v ok=%v", got, ok)
	}
	if got, ok := MatchFor("myapp", "myapp-1.2.0-amd64-windows.exe", "windows"); !ok || got.Kind != KindRaw {
		t.Fatalf("bare exe should match on windows, got %+v ok=%v", got, ok)
	}
	if _, ok := MatchFor("myapp", "myapp-1.2.0-amd64-linux.tar.gz", "windows"); ok {
		t.Fatal("linux asset must not match on windows")
	}
}

func TestPickPrefersArchive(t *testing.T) {
	names := []string{
		"LICENSE",
		"myapp-1.2.0-amd64-linux",
		"myapp-1.2.0-amd64-linux.tar.gz",
	}
	got, ok := PickFor("myapp", names, "linux")
	if !ok {
		t.Fatal("expected a match")
	}
	if got.Kind != KindTarGz {
		t.Fatalf("kind = %q, want archive to win over raw", got.Kind)
	}
}

func TestPickNoMatch(t *testing.T) {
	if _, ok := PickFor("myapp", []string{"Source code (tar.gz)", "myapp.tar.gz"}, "linux"); ok {
		t.Fatal("expected no match for source archives")
	}
}

func TestPattern(t *testing.T) {
	if got := Pattern("myapp", "windows"); got != "myapp-{version}-amd64-windows.zip" {
		t.Fatalf("windows pattern = %q", got)
	}
	if got := Pattern("myapp", "linux"); got != "myapp-{version}-amd64-linux.tar.gz" {
		t.Fatalf("linux pattern = %q", got)
	}
}

func TestNoReleaseReasonNamesPatternAndExample(t *testing.T) {
	got := NoReleaseReason("myapp", "linux")
	for _, want := range []string{
		"myapp-{version}-amd64-linux.tar.gz",
		"myapp-1.0.0-amd64-linux.tar.gz",
		"pre-release",
		"draft",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("NoReleaseReason() = %q, want it to mention %q", got, want)
		}
	}
}

func TestMatchForDegenerateNames(t *testing.T) {
	// Names where the repo prefix and the platform suffix overlap, or nearly
	// do, must be rejected rather than panicking.
	cases := []struct {
		repo string
		file string
		goos string
	}{
		{"x", "x-linux", "linux"},
		{"x", "x-linux.tar.gz", "linux"},
		{"myapp", "myapp-linux", "linux"},
		{"a", "a-windows.zip", "windows"},
		{"a", "a-windows.exe", "windows"},
		{"koa", "koa-", "linux"},
		{"koa", "koa-amd64-linux", "linux"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			if got, ok := MatchFor(tc.repo, tc.file, tc.goos); ok {
				t.Fatalf("MatchFor(%q, %q) matched unexpectedly: %+v", tc.repo, tc.file, got)
			}
		})
	}
}

func TestMatchPreservesVersionCase(t *testing.T) {
	got, ok := MatchFor("MyApp", "MyApp-V1.2.0-RC1-AMD64-LINUX.TAR.GZ", "linux")
	if !ok {
		t.Fatal("expected a case-insensitive match")
	}
	if got.Version != "V1.2.0-RC1" {
		t.Fatalf("version = %q, want the original casing preserved", got.Version)
	}
}

func TestMatchNonASCIINamesDoNotPanic(t *testing.T) {
	// Lower-casing some runes changes their byte length; the matcher must not
	// slice the original filename with lengths taken from the lowered one.
	names := []string{
		"İstanbul-1.0.0-amd64-linux.tar.gz",
		"KOA-İ-1.0.0-amd64-linux.tar.gz",
		"İ-amd64-linux",
	}
	repos := []string{"İstanbul", "koa-İ", "İ", "koa"}
	for _, repo := range repos {
		for _, name := range names {
			// The assertion is that this returns at all.
			MatchFor(repo, name, "linux")
			MatchFor(repo, name, "windows")
		}
	}
}
