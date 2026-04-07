package auth_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"tiny-file-watcher/client/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overrideTokensDir redirects token storage to a temp dir for the duration of
// the test, then restores $HOME.
func overrideHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestSaveAndLoadTokens_RoundTrip(t *testing.T) {
	overrideHome(t)

	want := auth.TokenSet{
		IDToken:      "my-id-token",
		RefreshToken: "my-refresh-token",
		Expiry:       time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	require.NoError(t, auth.SaveTokens(want))

	got, err := auth.LoadTokens()
	require.NoError(t, err)

	assert.Equal(t, want.IDToken, got.IDToken)
	assert.Equal(t, want.RefreshToken, got.RefreshToken)
	assert.True(t, want.Expiry.Equal(got.Expiry))
}

func TestSaveTokens_FilePermissions(t *testing.T) {
	home := overrideHome(t)

	require.NoError(t, auth.SaveTokens(auth.TokenSet{IDToken: "tok"}))

	info, err := os.Stat(filepath.Join(home, ".tfw", "tokens.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestLoadTokens_NotExist(t *testing.T) {
	overrideHome(t)

	_, err := auth.LoadTokens()
	require.Error(t, err)
	// The error must wrap os.ErrNotExist so callers can detect "not logged in".
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestLoadTokens_CorruptJSON(t *testing.T) {
	home := overrideHome(t)
	path := filepath.Join(home, ".tfw", "tokens.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0600))

	_, err := auth.LoadTokens()
	assert.ErrorContains(t, err, "parse tokens")
}

func TestClearTokens_RemovesFile(t *testing.T) {
	home := overrideHome(t)

	require.NoError(t, auth.SaveTokens(auth.TokenSet{IDToken: "tok"}))
	require.NoError(t, auth.ClearTokens())

	_, err := os.Stat(filepath.Join(home, ".tfw", "tokens.json"))
	assert.True(t, os.IsNotExist(err))
}

func TestClearTokens_IdempotentWhenMissing(t *testing.T) {
	overrideHome(t)
	// Calling ClearTokens when no file exists should not return an error.
	assert.NoError(t, auth.ClearTokens())
}

func TestSaveTokens_OmitsEmptyRefreshToken(t *testing.T) {
	home := overrideHome(t)

	require.NoError(t, auth.SaveTokens(auth.TokenSet{IDToken: "tok", Expiry: time.Now()}))

	raw, err := os.ReadFile(filepath.Join(home, ".tfw", "tokens.json"))
	require.NoError(t, err)
	// refresh_token field is tagged omitempty — must not appear in JSON.
	assert.NotContains(t, string(raw), "refresh_token")
}
