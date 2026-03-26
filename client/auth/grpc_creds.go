package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const errNotLoggedIn = "not logged in: run 'tfw login' first"

// TokenCredentials implements credentials.PerRPCCredentials.
// It loads the stored id_token and refreshes it when expired.
type TokenCredentials struct {
	tokenSource oauth2.TokenSource
	issuer      string
	clientID    string
}

// NewTokenCredentials creates a PerRPCCredentials that reads the stored
// id_token from disk, refreshing via the oauth2 refresh_token when needed.
// issuer and clientID are used only for token refresh.
func NewTokenCredentials(issuer, clientID string) (*TokenCredentials, error) {
	if _, err := LoadTokens(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf(errNotLoggedIn)
		}
		return nil, fmt.Errorf("load tokens: %w", err)
	}
	return &TokenCredentials{issuer: issuer, clientID: clientID}, nil
}

// GetRequestMetadata returns the Authorization header with the current id_token.
// It refreshes the token if it is expired and a refresh_token is available.
func (t *TokenCredentials) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	ts, err := LoadTokens()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf(errNotLoggedIn)
		}
		return nil, err
	}

	// Refresh if expired (with a 10-second buffer).
	if time.Now().After(ts.Expiry.Add(-10*time.Second)) && ts.RefreshToken != "" {
		newIDToken, newTS, refreshErr := refreshIDToken(ctx, t.issuer, t.clientID, ts.RefreshToken)
		if refreshErr != nil {
			return nil, fmt.Errorf("token refresh failed: %w", refreshErr)
		}
		if err := SaveTokens(newTS); err != nil {
			// Non-fatal — we still have a valid token for this request.
			fmt.Printf("Warning: failed to save refreshed tokens: %v\n", err)
		}
		return map[string]string{"authorization": "Bearer " + newIDToken}, nil
	}

	if ts.IDToken == "" {
		return nil, fmt.Errorf(errNotLoggedIn)
	}
	return map[string]string{"authorization": "Bearer " + ts.IDToken}, nil
}

// RequireTransportSecurity returns false because TLS is terminated at the
// reverse proxy; the CLI-to-proxy leg is already encrypted.
func (t *TokenCredentials) RequireTransportSecurity() bool { return false }

// refreshIDToken exchanges a refresh token for a new id_token using the
// oauth2 device-client configuration.
func refreshIDToken(ctx context.Context, issuer, clientID, refreshToken string) (string, TokenSet, error) {
	oidcProvider, err := gooidc.NewProvider(ctx, issuer)
	if err != nil {
		return "", TokenSet{}, fmt.Errorf("oidc discovery: %w", err)
	}
	tokenURL := oidcProvider.Endpoint().TokenURL
	cfg := oauth2.Config{
		ClientID: clientID,
		Endpoint: oauth2.Endpoint{
			TokenURL: tokenURL,
		},
	}

	src := cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	token, err := src.Token()
	if err != nil {
		return "", TokenSet{}, err
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return "", TokenSet{}, fmt.Errorf("no id_token in refresh response")
	}

	ts := TokenSet{
		IDToken:      rawIDToken,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry,
	}
	return rawIDToken, ts, nil
}
