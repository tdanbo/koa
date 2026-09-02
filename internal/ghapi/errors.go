package ghapi

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

// Error is a structured GitHub API failure. The UI branches on the flags here
// to show the right banner rather than a bare status code (PRD §5.5, §7).
type Error struct {
	StatusCode int
	Message    string
	URL        string
	// SSO is set when an org enforces SAML SSO and this token has not been
	// authorized for it (PRD §7). SSOURL points at the authorization page.
	SSO    bool
	SSOURL string
	// RateLimited is set when the request was rejected for exceeding a limit.
	RateLimited bool
	ResetAt     time.Time
	// Unauthorized is set when the token is missing, revoked or expired.
	Unauthorized bool
	// NotFound distinguishes "no such repo/release" from a real failure.
	NotFound bool
}

func (e *Error) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("github: %s (%d)", e.Message, e.StatusCode)
	}
	return fmt.Sprintf("github: request failed with status %d", e.StatusCode)
}

// IsNotFound reports whether err is a 404 from the GitHub API.
func IsNotFound(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.NotFound
}

// IsUnauthorized reports whether err means the token is no longer usable.
func IsUnauthorized(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Unauthorized
}

var ssoURLPattern = regexp.MustCompile(`url=([^;,\s]+)`)

// newError builds an Error from a response, using headers to tell the
// SSO / rate-limit / plain-403 cases apart.
func newError(resp *http.Response, message string) *Error {
	e := &Error{
		StatusCode: resp.StatusCode,
		Message:    message,
		URL:        resp.Request.URL.String(),
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		e.Unauthorized = true
		if e.Message == "" {
			e.Message = "the stored GitHub token was rejected"
		}
	case http.StatusNotFound:
		e.NotFound = true
	}

	if sso := resp.Header.Get("X-GitHub-SSO"); sso != "" {
		e.SSO = true
		if m := ssoURLPattern.FindStringSubmatch(sso); len(m) == 2 {
			e.SSOURL = m[1]
		}
	}

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		if resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.Header.Get("Retry-After") != "" {
			e.RateLimited = true
			if v := resp.Header.Get("X-RateLimit-Reset"); v != "" {
				if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
					e.ResetAt = time.Unix(secs, 0)
				}
			}
		}
	}

	return e
}
