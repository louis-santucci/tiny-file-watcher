package auth_test

import (
	"context"
	"testing"
	"time"

	"tiny-file-watcher/client/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTokenCredentials_NotLoggedIn(t *testing.T) {
	overrideHome(t) // empty temp dir — no tokens file

	_, err := auth.NewTokenCredentials("https://issuer.example", "client-id")
	require.Error(t, err)
	assert.ErrorContains(t, err, "not logged in")
}

func TestNewTokenCredentials_OK(t *testing.T) {
	overrideHome(t)
	require.NoError(t, auth.SaveTokens(auth.TokenSet{
		IDToken: "tok",
		Expiry:  time.Now().Add(time.Hour),
	}))

	creds, err := auth.NewTokenCredentials("https://issuer.example", "client-id")
	require.NoError(t, err)
	assert.NotNil(t, creds)
}

func TestGetRequestMetadata_ValidToken(t *testing.T) {
	overrideHome(t)
	require.NoError(t, auth.SaveTokens(auth.TokenSet{
		IDToken: "my-id-token",
		Expiry:  time.Now().Add(time.Hour),
	}))

	creds, err := auth.NewTokenCredentials("https://issuer.example", "client-id")
	require.NoError(t, err)

	md, err := creds.GetRequestMetadata(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Bearer my-id-token", md["authorization"])
}

func TestGetRequestMetadata_NoTokensFile(t *testing.T) {
	overrideHome(t)
	// Save tokens so NewTokenCredentials succeeds, then delete the file
	// to simulate the file being removed between construction and the RPC call.
	require.NoError(t, auth.SaveTokens(auth.TokenSet{IDToken: "tok", Expiry: time.Now().Add(time.Hour)}))

	creds, err := auth.NewTokenCredentials("https://issuer.example", "client-id")
	require.NoError(t, err)

	require.NoError(t, auth.ClearTokens())

	_, err = creds.GetRequestMetadata(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "not logged in")
}

func TestGetRequestMetadata_EmptyIDToken(t *testing.T) {
	overrideHome(t)
	// Token file exists but id_token is empty and not expired.
	require.NoError(t, auth.SaveTokens(auth.TokenSet{
		IDToken: "",
		Expiry:  time.Now().Add(time.Hour),
	}))

	creds, err := auth.NewTokenCredentials("https://issuer.example", "client-id")
	require.NoError(t, err)

	_, err = creds.GetRequestMetadata(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "not logged in")
}

func TestRequireTransportSecurity_ReturnsFalse(t *testing.T) {
	overrideHome(t)
	require.NoError(t, auth.SaveTokens(auth.TokenSet{IDToken: "tok", Expiry: time.Now().Add(time.Hour)}))

	creds, err := auth.NewTokenCredentials("https://issuer.example", "client-id")
	require.NoError(t, err)
	assert.False(t, creds.RequireTransportSecurity())
}
