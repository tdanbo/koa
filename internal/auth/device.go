package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Scopes are what koa's Device Flow asks for: `repo` to read private repos and
// their release assets, `read:org` to learn which orgs to search (PRD §7, §18).
var Scopes = []string{"repo", "read:org"}

// ScopeString renders the scopes for display and for the token request.
func ScopeString() string { return strings.Join(Scopes, " ") }

// Device flow endpoints. These live on github.com, not the API host.
const (
	deviceCodeURL  = "https://github.com/login/device/code"
	accessTokenURL = "https://github.com/login/oauth/access_token"
	// VerificationFallbackURL is where the user types the code if the browser
	// could not be opened for them.
	VerificationFallbackURL = "https://github.com/login/device"
)

// ErrClientIDMissing means koa was built without an OAuth App Client ID, so
// Device Flow cannot start (PRD §19 — registering the app is a manual step).
var ErrClientIDMissing = errors.New("no GitHub OAuth client ID is configured in this build")

// Device flow terminal failures, surfaced to the UI as-is.
var (
	ErrAccessDenied = errors.New("the sign-in request was denied on github.com")
	ErrCodeExpired  = errors.New("the device code expired before it was approved")
)

// DeviceCode is what the user is shown while approving koa (PRD §5.5).
type DeviceCode struct {
	DeviceCode      string    `json:"-"`
	UserCode        string    `json:"userCode"`
	VerificationURI string    `json:"verificationUri"`
	ExpiresAt       time.Time `json:"expiresAt"`
	Interval        int       `json:"interval"`
}

// DeviceFlow runs GitHub's OAuth Device Flow.
type DeviceFlow struct {
	ClientID   string
	HTTPClient *http.Client
	// MinInterval floors how often Poll contacts GitHub. GitHub asks for five
	// seconds; tests lower it.
	MinInterval time.Duration
	// codeURL and tokenURL are overridable for tests.
	codeURL  string
	tokenURL string
}

// defaultPollInterval is GitHub's requested floor between token polls.
const defaultPollInterval = 5 * time.Second

// NewDeviceFlow returns a flow for the given OAuth App client ID.
func NewDeviceFlow(clientID string) *DeviceFlow {
	return &DeviceFlow{
		ClientID:    clientID,
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
		MinInterval: defaultPollInterval,
		codeURL:     deviceCodeURL,
		tokenURL:    accessTokenURL,
	}
}

// SetEndpoints overrides the GitHub endpoints. Used by tests.
func (d *DeviceFlow) SetEndpoints(code, token string) {
	d.codeURL, d.tokenURL = code, token
}

// RequestCode asks GitHub for a device code and the short user code to display.
func (d *DeviceFlow) RequestCode(ctx context.Context) (DeviceCode, error) {
	if d.ClientID == "" {
		return DeviceCode{}, ErrClientIDMissing
	}

	form := url.Values{
		"client_id": {d.ClientID},
		"scope":     {ScopeString()},
	}

	var payload struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
		Error           string `json:"error"`
		ErrorDesc       string `json:"error_description"`
	}
	if err := d.post(ctx, d.codeURL, form, &payload); err != nil {
		return DeviceCode{}, err
	}
	if payload.Error != "" {
		return DeviceCode{}, fmt.Errorf("github rejected the sign-in request: %s", describe(payload.Error, payload.ErrorDesc))
	}
	if payload.DeviceCode == "" || payload.UserCode == "" {
		return DeviceCode{}, errors.New("github returned an incomplete device code response")
	}

	interval := payload.Interval
	if interval < 1 {
		interval = 5
	}
	expires := payload.ExpiresIn
	if expires < 1 {
		expires = 900
	}
	verify := payload.VerificationURI
	if verify == "" {
		verify = VerificationFallbackURL
	}

	return DeviceCode{
		DeviceCode:      payload.DeviceCode,
		UserCode:        payload.UserCode,
		VerificationURI: verify,
		ExpiresAt:       time.Now().Add(time.Duration(expires) * time.Second),
		Interval:        interval,
	}, nil
}

// Poll waits for the user to approve the request on github.com and returns the
// resulting access token. It honours GitHub's interval and slow_down signals.
func (d *DeviceFlow) Poll(ctx context.Context, code DeviceCode) (string, error) {
	floor := d.MinInterval
	if floor <= 0 {
		floor = defaultPollInterval
	}
	interval := time.Duration(code.Interval) * time.Second
	if interval < floor {
		interval = floor
	}

	form := url.Values{
		"client_id":   {d.ClientID},
		"device_code": {code.DeviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}

		if !code.ExpiresAt.IsZero() && time.Now().After(code.ExpiresAt) {
			return "", ErrCodeExpired
		}

		var payload struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
			Scope       string `json:"scope"`
			Error       string `json:"error"`
			ErrorDesc   string `json:"error_description"`
		}
		if err := d.post(ctx, d.tokenURL, form, &payload); err != nil {
			return "", err
		}

		switch payload.Error {
		case "":
			if payload.AccessToken == "" {
				return "", errors.New("github returned an empty access token")
			}
			return payload.AccessToken, nil
		case "authorization_pending":
			// Keep waiting — the user has not approved yet.
		case "slow_down":
			// GitHub asks callers to back off by five seconds; keep the step
			// proportional when a caller has lowered the floor.
			interval += min(floor, defaultPollInterval)
		case "expired_token":
			return "", ErrCodeExpired
		case "access_denied":
			return "", ErrAccessDenied
		default:
			return "", fmt.Errorf("github sign-in failed: %s", describe(payload.Error, payload.ErrorDesc))
		}
	}
}

func (d *DeviceFlow) post(ctx context.Context, endpoint string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "koa-desktop")

	client := d.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("contact github: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("github returned status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode github response: %w", err)
	}
	return nil
}

// describe prefers GitHub's human description over its machine error code.
func describe(code, desc string) string {
	if desc != "" {
		return desc
	}
	return code
}
