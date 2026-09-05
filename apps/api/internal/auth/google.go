package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// ErrGoogleNotConfigured is returned when Google OAuth env vars are unset.
var ErrGoogleNotConfigured = errors.New("google oauth is not configured")

// GoogleConfig holds the settings needed to run the Google OAuth flow.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// GoogleUserInfo is the subset of Google's userinfo response we care about.
type GoogleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

func (c GoogleConfig) configured() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.RedirectURL != ""
}

func (c GoogleConfig) oauth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  c.RedirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

// AuthURL builds the Google consent screen URL for the given CSRF state.
func (c GoogleConfig) AuthURL(state string) (string, error) {
	if !c.configured() {
		return "", ErrGoogleNotConfigured
	}
	return c.oauth2Config().AuthCodeURL(state, oauth2.AccessTypeOnline), nil
}

// Exchange trades an authorization code for the authenticated user's profile.
func (c GoogleConfig) Exchange(ctx context.Context, code string) (GoogleUserInfo, error) {
	if !c.configured() {
		return GoogleUserInfo{}, ErrGoogleNotConfigured
	}

	token, err := c.oauth2Config().Exchange(ctx, code)
	if err != nil {
		return GoogleUserInfo{}, fmt.Errorf("exchange code: %w", err)
	}

	client := c.oauth2Config().Client(ctx, token)
	resp, err := client.Get("https://openidconnect.googleapis.com/v1/userinfo")
	if err != nil {
		return GoogleUserInfo{}, fmt.Errorf("fetch userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return GoogleUserInfo{}, fmt.Errorf("userinfo request failed: %s", resp.Status)
	}

	var info GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return GoogleUserInfo{}, fmt.Errorf("decode userinfo: %w", err)
	}
	if info.Sub == "" || info.Email == "" {
		return GoogleUserInfo{}, errors.New("incomplete userinfo response")
	}
	return info, nil
}
