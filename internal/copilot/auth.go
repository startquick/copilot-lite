package copilot

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/crmmc/copilotpi/internal/store"
)

const (
	// msClientID is the Microsoft Copilot SPA client ID (public client, no secret required).
	msClientID = "14638111-3389-403d-b206-a6a71d9f8f16"
	// msScope is the scope required to get a Copilot-usable token via this client.
	// Uses graph.microsoft.com default scope + offline_access for refresh token.
	msScope = "https://graph.microsoft.com//.default offline_access openid profile"
	// msTokenEndpoint is the Microsoft identity platform token endpoint.
	// 'common' supports both personal MSA (e.g. Gmail-linked) and organizational accounts.
	msTokenEndpoint = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	// msAuthEndpoint is the Microsoft identity platform authorization endpoint.
	msAuthEndpoint = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"

	// tokenRefreshThreshold is the window before expiry when the token is pre-emptively refreshed.
	tokenRefreshThreshold = 5 * time.Minute
)

// MSTokenResponse is the JSON response from the Microsoft token endpoint.
type MSTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds
	TokenType    string `json:"token_type"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

// PKCEParams holds the PKCE code verifier and challenge for an authorization flow.
type PKCEParams struct {
	State        string
	CodeVerifier string
	CodeChallenge string
}

// GeneratePKCE generates PKCE parameters for a new OAuth2 authorization flow.
func GeneratePKCE() (*PKCEParams, error) {
	// Generate state (16 random bytes, hex-encoded)
	stateBuf := make([]byte, 16)
	if _, err := rand.Read(stateBuf); err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	// Generate code verifier (64 random bytes, base64url-encoded per RFC 7636)
	verifierBuf := make([]byte, 64)
	if _, err := rand.Read(verifierBuf); err != nil {
		return nil, fmt.Errorf("generate verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBuf)

	// Compute code challenge = base64url(SHA-256(verifier))
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	state := base64.RawURLEncoding.EncodeToString(stateBuf)

	return &PKCEParams{
		State:         state,
		CodeVerifier:  verifier,
		CodeChallenge: challenge,
	}, nil
}

// GenerateAuthURL builds the Microsoft authorization URL for the given redirect URI and PKCE params.
func GenerateAuthURL(redirectURI string, pkce *PKCEParams) string {
	params := url.Values{
		"client_id":             {msClientID},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"scope":                 {msScope},
		"state":                 {pkce.State},
		"code_challenge":        {pkce.CodeChallenge},
		"code_challenge_method": {"S256"},
		"response_mode":         {"query"},
	}
	return msAuthEndpoint + "?" + params.Encode()
}

// ExchangeCode exchanges an authorization code for an access token and refresh token.
func ExchangeCode(ctx context.Context, code, verifier, redirectURI string) (*MSTokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {msClientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
		"scope":         {msScope},
	}
	return postTokenRequest(ctx, data)
}

// RefreshMSToken exchanges a refresh token for a new access token and refresh token.
func RefreshMSToken(ctx context.Context, refreshToken string) (*MSTokenResponse, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {msClientID},
		"refresh_token": {refreshToken},
		"scope":         {msScope},
	}
	return postTokenRequest(ctx, data)
}

func postTokenRequest(ctx context.Context, data url.Values) (*MSTokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, msTokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	var tok MSTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if tok.Error != "" {
		return nil, fmt.Errorf("token error %s: %s", tok.Error, tok.ErrorDesc)
	}
	return &tok, nil
}

// MSAuthProvider implements TokenProvider using stored OAuth2 credentials.
// It automatically refreshes the access token when it is within the refresh threshold.
type MSAuthProvider struct {
	tokenID   uint
	tokenStore OAuthTokenStore
}

// OAuthTokenStore defines what MSAuthProvider needs to read and write OAuth tokens.
type OAuthTokenStore interface {
	GetToken(ctx context.Context, id uint) (*store.Token, error)
	SaveOAuthTokens(ctx context.Context, tokenID uint, creds store.OAuthCredentials) error
}

// NewMSAuthProvider creates a new MSAuthProvider for the given token ID.
func NewMSAuthProvider(tokenID uint, ts OAuthTokenStore) *MSAuthProvider {
	return &MSAuthProvider{tokenID: tokenID, tokenStore: ts}
}

// AccessToken returns a valid access token, refreshing it if necessary.
// Implements the TokenProvider interface.
func (p *MSAuthProvider) AccessToken(ctx context.Context) (string, error) {
	tok, err := p.tokenStore.GetToken(ctx, p.tokenID)
	if err != nil {
		return "", fmt.Errorf("load token %d: %w", p.tokenID, err)
	}

	if tok.RefreshToken == "" {
		return "", ErrInvalidToken
	}

	// Check if access token is still valid.
	if tok.AccessToken != "" && tok.TokenExpiresAt != nil {
		if time.Until(*tok.TokenExpiresAt) > tokenRefreshThreshold {
			return tok.AccessToken, nil
		}
	}

	// Refresh the token.
	slog.Debug("copilot/auth: refreshing access token", "token_id", p.tokenID)
	resp, err := RefreshMSToken(ctx, tok.RefreshToken)
	if err != nil {
		// If refresh_token is expired/revoked, mark the token as expired.
		if strings.Contains(err.Error(), "invalid_grant") {
			slog.Warn("copilot/auth: refresh token invalid — marking expired", "token_id", p.tokenID)
			expiresAt := time.Now().Add(-1 * time.Second)
			_ = p.tokenStore.SaveOAuthTokens(ctx, p.tokenID, store.OAuthCredentials{
				TokenExpiresAt: &expiresAt,
			})
			return "", ErrInvalidToken
		}
		return "", fmt.Errorf("refresh access token: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
	newCreds := store.OAuthCredentials{
		AccessToken:    resp.AccessToken,
		RefreshToken:   resp.RefreshToken,
		TokenExpiresAt: &expiresAt,
	}
	if err := p.tokenStore.SaveOAuthTokens(ctx, p.tokenID, newCreds); err != nil {
		slog.Warn("copilot/auth: failed to persist refreshed tokens", "token_id", p.tokenID, "error", err)
	}

	return resp.AccessToken, nil
}
