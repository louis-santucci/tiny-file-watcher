package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	h, err := New(
		&mockWatcherService{},
		&mockFlushService{},
		&mockRedirectionService{},
		&mockFilterService{},
		OIDCConfig{},
	)
	require.NoError(t, err)
	return h
}

func TestNew_Success(t *testing.T) {
	h, err := New(
		&mockWatcherService{},
		&mockFlushService{},
		&mockRedirectionService{},
		&mockFilterService{},
		OIDCConfig{},
	)
	require.NoError(t, err)
	assert.NotNil(t, h)
	assert.NotNil(t, h.mux)
	assert.Len(t, h.tmpls, len(pages))
}

func TestNew_AllPagesTemplatesLoaded(t *testing.T) {
	h := newTestHandler(t)
	for _, page := range pages {
		_, ok := h.tmpls[page]
		assert.True(t, ok, "expected template %q to be loaded", page)
	}
}

func TestRender_UnknownTemplate(t *testing.T) {
	h := newTestHandler(t)

	w := httptest.NewRecorder()
	h.render(w, "nonexistent.html", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "template not found")
}

func TestServeHTTP_NotFound(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/unknown-route", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func newHandlerWithAuth(t *testing.T) *Handler {
	t.Helper()
	h, err := New(
		&mockWatcherService{},
		&mockFlushService{},
		&mockRedirectionService{},
		&mockFilterService{},
		OIDCConfig{},
	)
	require.NoError(t, err)
	// Activate session store without a real OIDC provider.
	h.sessions = newSessionStore()
	return h
}

func TestRequireAuth_RedirectsWhenNoSession(t *testing.T) {
	h := newHandlerWithAuth(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/auth/login", w.Header().Get("Location"))
}

func TestRequireAuth_AllowsRequestWithValidSession(t *testing.T) {
	h := newHandlerWithAuth(t)

	sessionID := h.sessions.create()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	reached := false
	h.requireAuth(func(rw http.ResponseWriter, r *http.Request) {
		reached = true
		rw.WriteHeader(http.StatusOK)
	})(w, req)

	assert.True(t, reached, "expected next handler to be called")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireAuth_RedirectsWithExpiredSession(t *testing.T) {
	h := newHandlerWithAuth(t)

	req := httptest.NewRequest(http.MethodGet, "/watchers", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "invalid-session-id"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/auth/login", w.Header().Get("Location"))
}
