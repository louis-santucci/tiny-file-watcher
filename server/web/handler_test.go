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
