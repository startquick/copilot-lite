package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/crmmc/copilotpi/internal/copilot"
	"github.com/crmmc/copilotpi/internal/store"
)

// ─── OAuth token store interface ─────────────────────────────────────────────

// OAuthAdminStore is the interface for OAuth operations on token records.
type OAuthAdminStore interface {
	GetToken(ctx context.Context, id uint) (*store.Token, error)
	SaveOAuthTokens(ctx context.Context, tokenID uint, creds store.OAuthCredentials) error
}

// ─── In-memory PKCE session store ────────────────────────────────────────────

// pkceSession holds the PKCE verifier and target token ID for a pending OAuth flow.
type pkceSession struct {
	TokenID     uint
	CodeVerifier string
	State        string
	RedirectURI  string
	CreatedAt    time.Time
}

// pkceSessionStore is a simple in-memory store for active PKCE sessions.
// Only one concurrent flow per server is supported (native-app pattern).
type pkceSessionStore struct {
	mu      sync.Mutex
	pending map[string]*pkceSession // state → session
}

var globalPKCEStore = &pkceSessionStore{
	pending: make(map[string]*pkceSession),
}

func (s *pkceSessionStore) set(state string, sess *pkceSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Evict stale sessions (older than 10 min)
	for k, v := range s.pending {
		if time.Since(v.CreatedAt) > 10*time.Minute {
			delete(s.pending, k)
		}
	}
	s.pending[state] = sess
}

func (s *pkceSessionStore) get(state string) (*pkceSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.pending[state]
	return sess, ok
}

func (s *pkceSessionStore) delete(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, state)
}

// ─── Local callback listener ─────────────────────────────────────────────────

// localCallbackResult holds the result from the local OAuth callback listener.
type localCallbackResult struct {
	Code  string
	State string
	Err   error
}

// startLocalCallbackListener starts a temporary HTTP listener on a random localhost port.
// It returns the actual port assigned and shuts down automatically after the first
// code is received or after timeout (60s).
func startLocalCallbackListener(resultCh chan<- localCallbackResult) (port int, err error) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, fmt.Errorf("listen on random port: %w", err)
	}
	port = ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux}

	var once sync.Once
	done := func(res localCallbackResult) {
		once.Do(func() {
			resultCh <- res
		})
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		code := q.Get("code")
		state := q.Get("state")
		oauthErr := q.Get("error")

		if oauthErr != "" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body><h2>Authentication Failed</h2><p>%s: %s</p><p>You may close this window.</p></body></html>",
				oauthErr, q.Get("error_description"))
			done(localCallbackResult{Err: fmt.Errorf("oauth error %s: %s", oauthErr, q.Get("error_description"))})
			go srv.Shutdown(context.Background()) //nolint:errcheck
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>
<h2>✅ Authentication Successful</h2>
<p>You have been successfully authenticated. You may close this window and return to the admin dashboard.</p>
</body></html>`)
		done(localCallbackResult{Code: code, State: state})
		go srv.Shutdown(context.Background()) //nolint:errcheck
	})

	go func() {
		_ = srv.Serve(ln)
	}()

	// Auto-shutdown after 60 seconds if no callback received.
	go func() {
		time.Sleep(60 * time.Second)
		_ = srv.Shutdown(context.Background())
		done(localCallbackResult{Err: fmt.Errorf("OAuth callback timed out after 60 seconds")})
	}()

	return port, nil
}


// ─── HTTP Handlers ────────────────────────────────────────────────────────────

// handleOAuthStart returns GET /admin/auth/copilot/start — initiates the OAuth2 login flow.
func handleOAuthStart(ts OAuthAdminStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.URL.Query().Get("token_id")
		tokenID, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil || tokenID == 0 {
			WriteError(w, http.StatusBadRequest, "invalid_request", "invalid_token_id", "token_id query parameter is required")
			return
		}

		// Verify token exists.
		if _, err := ts.GetToken(r.Context(), uint(tokenID)); err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "token_not_found", "Token not found")
			return
		}

		pkce, err := copilot.GeneratePKCE()
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "server_error", "pkce_error", "Failed to generate PKCE parameters")
			return
		}

		// Start local callback listener on a random port.
		resultCh := make(chan localCallbackResult, 1)
		actuaPort, err := startLocalCallbackListener(resultCh)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "server_error", "listener_error", "Failed to start callback listener: "+err.Error())
			return
		}

		redirectURI := fmt.Sprintf("http://localhost:%d/", actuaPort)
		authURL := copilot.GenerateAuthURL(redirectURI, pkce)

		// Store PKCE session keyed by state.
		globalPKCEStore.set(pkce.State, &pkceSession{
			TokenID:      uint(tokenID),
			CodeVerifier: pkce.CodeVerifier,
			State:        pkce.State,
			RedirectURI:  redirectURI,
			CreatedAt:    time.Now(),
		})

		// Launch a goroutine to handle the incoming callback and exchange the code.
		// Use context.Background() — r.Context() is cancelled as soon as the /start response is sent.
		go handleLocalCallback(context.Background(), resultCh, ts)

		slog.Info("oauth: flow started", "token_id", tokenID, "redirect_uri", redirectURI)

		WriteJSON(w, http.StatusOK, map[string]string{
			"auth_url":     authURL,
			"state":        pkce.State,
			"redirect_uri": redirectURI,
		})
	}
}


// handleLocalCallback processes the callback result from the local listener.
func handleLocalCallback(ctx context.Context, resultCh <-chan localCallbackResult, ts OAuthAdminStore) {
	res := <-resultCh
	if res.Err != nil {
		slog.Warn("oauth: local callback error", "error", res.Err)
		return
	}

	sess, ok := globalPKCEStore.get(res.State)
	if !ok {
		slog.Warn("oauth: no PKCE session found for state", "state", res.State)
		return
	}
	globalPKCEStore.delete(res.State)

	exchangeAndSave(ctx, res.Code, sess.CodeVerifier, sess.RedirectURI, sess.TokenID, ts)
}

// handleOAuthCallback returns POST /admin/auth/copilot/callback — manual callback URL paste fallback.
func handleOAuthCallback(ts OAuthAdminStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CallbackURL string `json:"callback_url"`
			TokenID     uint   `json:"token_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid_request", "invalid_json", "Invalid JSON in request body")
			return
		}
		if req.CallbackURL == "" || req.TokenID == 0 {
			WriteError(w, http.StatusBadRequest, "invalid_request", "missing_fields", "callback_url and token_id are required")
			return
		}

		// Parse code and state from callback URL.
		parsed, err := url.Parse(req.CallbackURL)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid_request", "invalid_url", "Cannot parse callback URL")
			return
		}
		q := parsed.Query()
		code := q.Get("code")
		state := q.Get("state")
		if code == "" {
			WriteError(w, http.StatusBadRequest, "invalid_request", "missing_code", "No authorization code found in callback URL")
			return
		}

		// Look up PKCE session by state.
		var sess *pkceSession
		var ok bool
		if state != "" {
			sess, ok = globalPKCEStore.get(state)
		}

		// Fallback: try to find any session for this token ID (in case state is missing).
		if !ok {
			globalPKCEStore.mu.Lock()
			for _, s := range globalPKCEStore.pending {
				if s.TokenID == req.TokenID {
					sess = s
					state = s.State
					ok = true
					break
				}
			}
			globalPKCEStore.mu.Unlock()
		}

		if !ok || sess == nil {
			WriteError(w, http.StatusBadRequest, "invalid_request", "no_session", "No active OAuth session found for this token")
			return
		}
		globalPKCEStore.delete(state)

		if err := exchangeAndSave(r.Context(), code, sess.CodeVerifier, sess.RedirectURI, req.TokenID, ts); err != nil {
			WriteError(w, http.StatusBadGateway, "upstream_error", "exchange_failed", err.Error())
			return
		}

		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "OAuth login successful"})
	}
}

// exchangeAndSave exchanges the authorization code for tokens and saves them.
func exchangeAndSave(ctx context.Context, code, verifier, redirectURI string, tokenID uint, ts OAuthAdminStore) error {
	slog.Info("oauth: exchanging authorization code", "token_id", tokenID)
	resp, err := copilot.ExchangeCode(ctx, code, verifier, redirectURI)
	if err != nil {
		slog.Warn("oauth: code exchange failed", "token_id", tokenID, "error", err)
		return fmt.Errorf("code exchange failed: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
	creds := store.OAuthCredentials{
		AccessToken:    resp.AccessToken,
		RefreshToken:   resp.RefreshToken,
		TokenExpiresAt: &expiresAt,
	}
	if err := ts.SaveOAuthTokens(ctx, tokenID, creds); err != nil {
		slog.Error("oauth: failed to save tokens", "token_id", tokenID, "error", err)
		return fmt.Errorf("persist tokens: %w", err)
	}

	slog.Info("oauth: tokens saved successfully", "token_id", tokenID, "expires_at", expiresAt)
	return nil
}

// ─── Route Registration ───────────────────────────────────────────────────────

// registerOAuthRoutes registers the OAuth admin routes on an *already-authenticated* chi router group.
func registerOAuthRoutes(r chi.Router, ts OAuthAdminStore, _ int) {
	r.Get("/auth/copilot/start", handleOAuthStart(ts))
	r.Post("/auth/copilot/callback", handleOAuthCallback(ts))
}

// getOAuthTokenStatus returns the OAuth status string for a token record.
func getOAuthTokenStatus(t *store.Token) string {
	if t.RefreshToken == "" {
		return "unauthenticated"
	}
	if t.TokenExpiresAt == nil {
		return "unauthenticated"
	}
	now := time.Now()
	if t.TokenExpiresAt.Before(now) {
		return "expired"
	}
	if t.TokenExpiresAt.Before(now.Add(time.Hour)) {
		return "expiring"
	}
	return "valid"
}
