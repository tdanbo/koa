package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/playdead/koa/internal/auth"
	"github.com/playdead/koa/internal/config"
	"github.com/playdead/koa/internal/ghapi"
)

// SignInWithGitHub starts the OAuth Device Flow. It returns the short code to
// display immediately and continues polling in the background, emitting
// EventAuth when the attempt resolves (PRD §7).
func (s *Service) SignInWithGitHub() (SignInPrompt, error) {
	if s.device.ClientID == "" {
		return SignInPrompt{}, errors.New("this build of Koa has no GitHub OAuth client ID — set KOA_GITHUB_CLIENT_ID or use Settings › Enter token")
	}

	s.CancelSignIn()

	code, err := s.device.RequestCode(s.context())
	if err != nil {
		return SignInPrompt{}, err
	}

	opened := s.OpenExternal(code.VerificationURI) == nil

	ctx, cancel := context.WithCancel(s.context())
	s.signInMu.Lock()
	s.signInCancel = cancel
	s.signInMu.Unlock()

	go func() {
		defer cancel()
		token, err := s.device.Poll(ctx, code)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				s.emit(EventAuth, AuthEvent{Status: "cancelled"})
				return
			}
			s.emit(EventAuth, AuthEvent{Status: "failed", Error: err.Error()})
			return
		}
		account, err := s.adoptToken(token, auth.SourceDevice, auth.ScopeString())
		if err != nil {
			s.emit(EventAuth, AuthEvent{Status: "failed", Error: err.Error()})
			return
		}
		s.invalidateDiscovery()
		s.emit(EventAuth, AuthEvent{Status: "signed-in", Account: account})
	}()

	return SignInPrompt{
		UserCode:        code.UserCode,
		VerificationURI: code.VerificationURI,
		ExpiresAt:       code.ExpiresAt,
		BrowserOpened:   opened,
	}, nil
}

// CancelSignIn abandons an in-flight Device Flow attempt.
func (s *Service) CancelSignIn() {
	s.signInMu.Lock()
	cancel := s.signInCancel
	s.signInCancel = nil
	s.signInMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// SignInWithToken stores a hand-pasted token, validating it first (PRD §7).
func (s *Service) SignInWithToken(token string) (Account, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Account{}, errors.New("enter a GitHub token")
	}
	return s.adoptToken(token, auth.SourceManual, "")
}

// adoptToken validates a token with one lightweight API call, stores it, and
// updates the account view (PRD §7).
func (s *Service) adoptToken(token string, source auth.Source, scopes string) (Account, error) {
	probe := s.gh.Clone(token)
	user, err := probe.CurrentUser(s.context())
	if err != nil {
		if ghapi.IsUnauthorized(err) {
			return Account{}, errors.New("GitHub rejected that token — check that it is valid and not expired")
		}
		return Account{}, fmt.Errorf("could not verify the token: %w", err)
	}

	where, err := s.creds.Save(auth.Credential{
		Token:  token,
		Source: source,
		Scopes: scopes,
		Login:  user.Login,
	})
	if err != nil {
		return Account{}, err
	}

	s.gh.SetToken(token)
	account := Account{
		SignedIn:               true,
		Login:                  user.Login,
		Name:                   user.Name,
		AvatarURL:              user.AvatarURL,
		Source:                 string(source),
		Scopes:                 scopes,
		TokenStorage:           auth.StorageLabel(where),
		UsingPlaintextFallback: where == auth.StorageFile,
		PlaintextPath:          config.Display(s.creds.FallbackPath()),
	}
	s.setAccount(account)
	s.invalidateDiscovery()
	return account, nil
}

// SignOut clears the stored credential and empties Discover (PRD §13).
func (s *Service) SignOut() error {
	s.CancelSignIn()
	if err := s.creds.Delete(); err != nil {
		return err
	}
	s.gh.SetToken("")
	s.setAccount(Account{})
	s.invalidateDiscovery()
	return nil
}

// RefreshAccount re-reads the account from GitHub, which also confirms the
// stored token still works.
func (s *Service) RefreshAccount() (Account, error) {
	current := s.Account()
	if !current.SignedIn {
		return current, nil
	}
	user, err := s.gh.CurrentUser(s.context())
	if err != nil {
		if ghapi.IsUnauthorized(err) {
			_ = s.SignOut()
			return Account{}, errors.New("the stored GitHub token is no longer valid — sign in again")
		}
		return current, err
	}
	current.Login = user.Login
	current.Name = user.Name
	current.AvatarURL = user.AvatarURL
	s.setAccount(current)
	return current, nil
}
