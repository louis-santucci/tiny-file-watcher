package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"net/http"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	sessionCookieName = "tfw_session"
	stateCookieName   = "tfw_oidc_state"
	sessionTTL        = 12 * time.Hour
)

// OIDCConfig holds the OIDC provider settings read from config.
type OIDCConfig struct {
	Enabled      bool
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// authProvider wraps the OIDC provider and OAuth2 config.
type authProvider struct {
	cfg      OIDCConfig
	verifier *gooidc.IDTokenVerifier
	oauth2   oauth2.Config
}

// newAuthProvider initialises the OIDC provider via discovery.
func newAuthProvider(ctx context.Context, cfg OIDCConfig) (*authProvider, error) {
	provider, err := gooidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, err
	}

	oauth2Cfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{gooidc.ScopeOpenID, "profile", "email"},
	}

	verifier := provider.Verifier(&gooidc.Config{ClientID: cfg.ClientID})

	return &authProvider{
		cfg:      cfg,
		verifier: verifier,
		oauth2:   oauth2Cfg,
	}, nil
}

// sessionStore is a simple in-memory session store.
type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]time.Time // session ID → expiry
}

func newSessionStore() *sessionStore {
	s := &sessionStore{sessions: make(map[string]time.Time)}
	go s.reapLoop()
	return s
}

func (s *sessionStore) create() string {
	id := randomToken(32)
	s.mu.Lock()
	s.sessions[id] = time.Now().Add(sessionTTL)
	s.mu.Unlock()
	return id
}

func (s *sessionStore) valid(id string) bool {
	s.mu.RLock()
	expiry, ok := s.sessions[id]
	s.mu.RUnlock()
	return ok && time.Now().Before(expiry)
}

func (s *sessionStore) delete(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// reapLoop periodically removes expired sessions.
func (s *sessionStore) reapLoop() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for id, expiry := range s.sessions {
			if now.After(expiry) {
				delete(s.sessions, id)
			}
		}
		s.mu.Unlock()
	}
}

// requireAuth is middleware that redirects unauthenticated requests to /auth/login.
func (h *Handler) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.isAuthenticated(r) {
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

func (h *Handler) isAuthenticated(r *http.Request) bool {
	if h.sessions == nil {
		return true // OIDC disabled
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	return h.sessions.valid(cookie.Value)
}

// handleLoginPage renders the login page.
func (h *Handler) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if h.isAuthenticated(r) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	h.render(w, "login.html", nil)
}

// handleLogin redirects the browser to the OIDC provider's authorization endpoint.
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	state := randomToken(16)
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/auth/callback",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300,
	})
	http.Redirect(w, r, h.auth.oauth2.AuthCodeURL(state), http.StatusFound)
}

// handleCallback handles the OIDC authorization code callback.
func (h *Handler) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Verify state to prevent CSRF.
	stateCookie, err := r.Cookie(stateCookieName)
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	// Clear the state cookie.
	http.SetCookie(w, &http.Cookie{
		Name:   stateCookieName,
		Path:   "/auth/callback",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	token, err := h.auth.oauth2.Exchange(r.Context(), code)
	if err != nil {
		slog.Error("oidc: token exchange failed", "err", err)
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token in response", http.StatusInternalServerError)
		return
	}

	if _, err := h.auth.verifier.Verify(r.Context(), rawIDToken); err != nil {
		slog.Error("oidc: id_token verification failed", "err", err)
		http.Error(w, "id_token verification failed", http.StatusUnauthorized)
		return
	}

	sessionID := h.sessions.create()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

// handleLogout invalidates the session and redirects to the login page.
func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		h.sessions.delete(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookieName,
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/auth/login", http.StatusFound)
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
