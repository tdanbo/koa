package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCredentialFileFallbackRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewCredentialStore(dir)

	// Write the fallback file directly: the OS keyring is not available in a
	// headless test environment, and Load must still find the credential.
	raw, _ := json.Marshal(Credential{Token: "ghp_abc", Source: SourceDevice, Login: "m-halvorsen"})
	if err := os.WriteFile(s.FallbackPath(), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Token != "ghp_abc" || got.Source != SourceDevice || got.Login != "m-halvorsen" {
		t.Fatalf("credential = %+v", got)
	}
	if got.Where != StorageFile {
		t.Fatalf("Where = %q, want %q", got.Where, StorageFile)
	}
}

func TestLoadNoCredential(t *testing.T) {
	s := NewCredentialStore(t.TempDir())
	if _, err := s.Load(); !errors.Is(err, ErrNoCredential) {
		t.Fatalf("want ErrNoCredential, got %v", err)
	}
}

func TestDecodeTolerateBareToken(t *testing.T) {
	cred, err := decode("  ghp_bare  ")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cred.Token != "ghp_bare" || cred.Source != SourceManual {
		t.Fatalf("credential = %+v", cred)
	}
	if _, err := decode("   "); !errors.Is(err, ErrNoCredential) {
		t.Fatalf("empty should report no credential, got %v", err)
	}
	if _, err := decode(`{"token":""}`); !errors.Is(err, ErrNoCredential) {
		t.Fatalf("empty token should report no credential, got %v", err)
	}
}

func TestDeleteRemovesFallbackFile(t *testing.T) {
	dir := t.TempDir()
	s := NewCredentialStore(dir)
	if err := os.WriteFile(s.FallbackPath(), []byte(`{"token":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(s.FallbackPath()); !os.IsNotExist(err) {
		t.Fatal("fallback file survived Delete")
	}
	if err := s.Delete(); err != nil {
		t.Fatalf("second Delete should be a no-op: %v", err)
	}
}

func TestFallbackFileIsNotWorldReadable(t *testing.T) {
	dir := t.TempDir()
	s := NewCredentialStore(dir)
	where, err := s.Save(Credential{Token: "ghp_secret", Source: SourceManual})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if where != StorageFile {
		t.Skip("an OS keyring is available here; the file path is not exercised")
	}
	info, err := os.Stat(s.FallbackPath())
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("credential file mode = %o, want 600", perm)
	}
	if base := filepath.Base(s.FallbackPath()); base != "credentials.json" {
		t.Errorf("fallback file name = %q", base)
	}
}

func TestRequestCodeWithoutClientID(t *testing.T) {
	d := NewDeviceFlow("")
	if _, err := d.RequestCode(context.Background()); !errors.Is(err, ErrClientIDMissing) {
		t.Fatalf("want ErrClientIDMissing, got %v", err)
	}
}

func TestRequestCodeSendsScopes(t *testing.T) {
	var gotScope, gotClient string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		gotScope, gotClient = r.Form.Get("scope"), r.Form.Get("client_id")
		fmt.Fprint(w, `{"device_code":"dc","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":900,"interval":5}`)
	}))
	defer srv.Close()

	d := NewDeviceFlow("client-123")
	d.SetEndpoints(srv.URL, srv.URL)

	code, err := d.RequestCode(context.Background())
	if err != nil {
		t.Fatalf("RequestCode: %v", err)
	}
	if gotClient != "client-123" {
		t.Errorf("client_id = %q", gotClient)
	}
	if gotScope != "repo read:org" {
		t.Errorf("scope = %q, want the two scopes from the PRD", gotScope)
	}
	if code.UserCode != "ABCD-1234" || code.DeviceCode != "dc" {
		t.Errorf("code = %+v", code)
	}
	if code.Interval != 5 || code.ExpiresAt.Before(time.Now()) {
		t.Errorf("timing fields = %+v", code)
	}
}

func TestPollHandlesPendingSlowDownThenSuccess(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			fmt.Fprint(w, `{"error":"authorization_pending"}`)
		case 2:
			fmt.Fprint(w, `{"error":"slow_down"}`)
		default:
			fmt.Fprint(w, `{"access_token":"ghp_ok","token_type":"bearer","scope":"repo,read:org"}`)
		}
	}))
	defer srv.Close()

	d := NewDeviceFlow("c")
	d.SetEndpoints(srv.URL, srv.URL)
	d.MinInterval = 5 * time.Millisecond

	token, err := d.Poll(context.Background(), DeviceCode{DeviceCode: "dc", ExpiresAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if token != "ghp_ok" {
		t.Fatalf("token = %q", token)
	}
	if calls.Load() < 3 {
		t.Fatalf("expected the pending and slow_down responses to be retried, got %d calls", calls.Load())
	}
}

func TestPollTerminalErrors(t *testing.T) {
	cases := []struct {
		body string
		want error
	}{
		{`{"error":"access_denied"}`, ErrAccessDenied},
		{`{"error":"expired_token"}`, ErrCodeExpired},
	}
	for _, tc := range cases {
		t.Run(tc.body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()

			d := NewDeviceFlow("c")
			d.SetEndpoints(srv.URL, srv.URL)
			d.MinInterval = 5 * time.Millisecond
			_, err := d.Poll(context.Background(), DeviceCode{DeviceCode: "dc", ExpiresAt: time.Now().Add(time.Minute)})
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestPollStopsWhenCodeExpires(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"error":"authorization_pending"}`)
	}))
	defer srv.Close()

	d := NewDeviceFlow("c")
	d.SetEndpoints(srv.URL, srv.URL)
	d.MinInterval = 5 * time.Millisecond
	_, err := d.Poll(context.Background(), DeviceCode{DeviceCode: "dc", ExpiresAt: time.Now().Add(-time.Second)})
	if !errors.Is(err, ErrCodeExpired) {
		t.Fatalf("got %v, want ErrCodeExpired", err)
	}
}

func TestPollRespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"error":"authorization_pending"}`)
	}))
	defer srv.Close()

	d := NewDeviceFlow("c")
	d.SetEndpoints(srv.URL, srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := d.Poll(ctx, DeviceCode{DeviceCode: "dc", ExpiresAt: time.Now().Add(time.Minute)})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want the context error", err)
	}
}

func TestStorageLabel(t *testing.T) {
	if got := StorageLabel(StorageFile); !strings.Contains(got, "plaintext") {
		t.Errorf("file label = %q", got)
	}
	if got := StorageLabel(StorageKeyring); strings.Contains(got, "plaintext") {
		t.Errorf("keyring label = %q", got)
	}
}
