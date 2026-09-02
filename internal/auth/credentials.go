// Package auth handles GitHub sign-in: the OAuth Device Flow and the storage
// of the resulting token (PRD §7).
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

// keyringService and keyringUser identify koa's entry in the OS credential
// store.
const (
	keyringService = "koa"
	keyringUser    = "github-token"
)

// Source records how a token was obtained, so Settings can describe it.
type Source string

const (
	// SourceDevice is the OAuth Device Flow (PRD §7).
	SourceDevice Source = "device"
	// SourceManual is a hand-pasted token — typically a fine-grained PAT.
	SourceManual Source = "manual"
)

// Storage records where the token physically lives.
type Storage string

const (
	StorageKeyring Storage = "keyring"
	StorageFile    Storage = "file"
)

// Credential is a stored GitHub token plus the provenance the UI displays.
type Credential struct {
	Token  string  `json:"token"`
	Source Source  `json:"source"`
	Scopes string  `json:"scopes"`
	Login  string  `json:"login"`
	Where  Storage `json:"-"`
}

// ErrNoCredential means the user has not signed in yet.
var ErrNoCredential = errors.New("no stored github credential")

// CredentialStore persists the token in the OS credential store, falling back
// to a 0600 file when no keyring service is available (PRD §4, §16).
type CredentialStore struct {
	fallbackPath string
	// keyringWorks caches whether the OS keyring responded, so the UI can warn
	// once rather than probing repeatedly.
	keyringWorks *bool
}

// NewCredentialStore returns a store whose plaintext fallback lives in dir.
func NewCredentialStore(dir string) *CredentialStore {
	return &CredentialStore{fallbackPath: filepath.Join(dir, "credentials.json")}
}

// Load returns the stored credential, preferring the keyring.
func (s *CredentialStore) Load() (Credential, error) {
	if raw, err := keyring.Get(keyringService, keyringUser); err == nil {
		cred, err := decode(raw)
		if err == nil {
			cred.Where = StorageKeyring
			s.markKeyring(true)
			return cred, nil
		}
	} else if !errors.Is(err, keyring.ErrNotFound) {
		s.markKeyring(false)
	} else {
		s.markKeyring(true)
	}

	raw, err := os.ReadFile(s.fallbackPath)
	if errors.Is(err, fs.ErrNotExist) {
		return Credential{}, ErrNoCredential
	}
	if err != nil {
		return Credential{}, fmt.Errorf("read credential file: %w", err)
	}
	cred, err := decode(string(raw))
	if err != nil {
		return Credential{}, err
	}
	if cred.Token == "" {
		return Credential{}, ErrNoCredential
	}
	cred.Where = StorageFile
	return cred, nil
}

// Save writes the credential, reporting where it ended up so the UI can flag
// the plaintext fallback.
func (s *CredentialStore) Save(cred Credential) (Storage, error) {
	raw, err := json.Marshal(cred)
	if err != nil {
		return "", fmt.Errorf("encode credential: %w", err)
	}

	if err := keyring.Set(keyringService, keyringUser, string(raw)); err == nil {
		s.markKeyring(true)
		// Clear any earlier plaintext copy now that the keyring works.
		_ = os.Remove(s.fallbackPath)
		return StorageKeyring, nil
	}
	s.markKeyring(false)

	if err := os.MkdirAll(filepath.Dir(s.fallbackPath), 0o700); err != nil {
		return "", fmt.Errorf("create credential directory: %w", err)
	}
	if err := os.WriteFile(s.fallbackPath, raw, 0o600); err != nil {
		return "", fmt.Errorf("write credential file: %w", err)
	}
	return StorageFile, nil
}

// Delete removes the credential from both locations (PRD §13, sign out).
func (s *CredentialStore) Delete() error {
	var errs []error
	if err := keyring.Delete(keyringService, keyringUser); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		// A missing keyring service is not a failure to sign out.
		if s.KeyringAvailable() {
			errs = append(errs, fmt.Errorf("clear keyring entry: %w", err))
		}
	}
	if err := os.Remove(s.fallbackPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		errs = append(errs, fmt.Errorf("remove credential file: %w", err))
	}
	return errors.Join(errs...)
}

// KeyringAvailable reports whether the OS credential store answered. It is
// false before any Load or Save has been attempted.
func (s *CredentialStore) KeyringAvailable() bool {
	return s.keyringWorks != nil && *s.keyringWorks
}

// FallbackPath is the plaintext file's location, for the warning banner.
func (s *CredentialStore) FallbackPath() string { return s.fallbackPath }

func (s *CredentialStore) markKeyring(ok bool) { s.keyringWorks = &ok }

// StorageLabel names where a token is kept, in the words the reference uses
// ("token in Credential Manager", PRD §5.3 Settings).
func StorageLabel(where Storage) string {
	if where == StorageFile {
		return "a plaintext file"
	}
	return platformKeyringLabel()
}

// decode reads a stored credential, tolerating an entry that holds a bare
// token string from an older koa.
func decode(raw string) (Credential, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Credential{}, ErrNoCredential
	}
	if !strings.HasPrefix(trimmed, "{") {
		return Credential{Token: trimmed, Source: SourceManual}, nil
	}
	var cred Credential
	if err := json.Unmarshal([]byte(trimmed), &cred); err != nil {
		return Credential{}, fmt.Errorf("parse stored credential: %w", err)
	}
	if cred.Token == "" {
		return Credential{}, ErrNoCredential
	}
	return cred, nil
}
