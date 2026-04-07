package cmd

import (
	"context"
	"fmt"
	"time"

	cfg "tiny-file-watcher/client/config"

	"tiny-file-watcher/client/auth"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the OIDC provider via device flow",
	// Skip the PersistentPreRunE (dial) for login/logout — no gRPC needed.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cfg.LoadClientConfig()
		if err != nil {
			return err
		}
		issuer, _ := cfg.String("oidc.issuer")
		clientID, _ := cfg.String("oidc.device-client-id")

		ctx := context.Background()
		provider, err := gooidc.NewProvider(ctx, issuer)
		if err != nil {
			return fmt.Errorf("oidc discovery: %w", err)
		}

		oauth2Cfg := oauth2.Config{
			ClientID: clientID,
			Endpoint: provider.Endpoint(),
			Scopes:   []string{gooidc.ScopeOpenID, "profile", "email"},
		}

		deviceAuth, err := oauth2Cfg.DeviceAuth(ctx)
		if err != nil {
			return fmt.Errorf("device auth request: %w", err)
		}

		fmt.Printf("Open the following URL to log in:\n\n  %s\n\nAnd enter code: %s\n\n",
			deviceAuth.VerificationURIComplete, deviceAuth.UserCode)

		token, err := oauth2Cfg.DeviceAccessToken(ctx, deviceAuth)
		if err != nil {
			return fmt.Errorf("device access token: %w", err)
		}

		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok || rawIDToken == "" {
			return fmt.Errorf("no id_token in response; ensure the OIDC client has openid scope and device flow enabled")
		}

		expiry := token.Expiry
		if expiry.IsZero() {
			expiry = time.Now().Add(5 * time.Minute)
		}

		if err := auth.SaveTokens(auth.TokenSet{
			IDToken:      rawIDToken,
			RefreshToken: token.RefreshToken,
			Expiry:       expiry,
		}); err != nil {
			return fmt.Errorf("save tokens: %w", err)
		}

		fmt.Println("Login successful. Tokens saved to ~/.tfw/tokens.json")
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored authentication tokens",
	// Skip the PersistentPreRunE (dial) for login/logout — no gRPC needed.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := auth.ClearTokens(); err != nil {
			return fmt.Errorf("clear tokens: %w", err)
		}
		fmt.Println("Logged out. Tokens removed.")
		return nil
	},
}
