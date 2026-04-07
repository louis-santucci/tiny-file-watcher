package interceptor

import (
	"context"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
)

// TokenVerifier validates a raw ID token string.
type TokenVerifier interface {
	Verify(ctx context.Context, rawToken string) error
}

// noopVerifier accepts every token (used when OIDC is disabled).
type noopVerifier struct{}

func (noopVerifier) Verify(_ context.Context, _ string) error { return nil }

// NewNoopVerifier returns a TokenVerifier that always succeeds.
func NewNoopVerifier() TokenVerifier { return noopVerifier{} }

// OIDCTokenVerifier wraps a go-oidc IDTokenVerifier.
type OIDCTokenVerifier struct {
	v *gooidc.IDTokenVerifier
}

// NewOIDCTokenVerifier creates a verifier from a go-oidc IDTokenVerifier.
func NewOIDCTokenVerifier(v *gooidc.IDTokenVerifier) *OIDCTokenVerifier {
	return &OIDCTokenVerifier{v: v}
}

// Verify validates the raw id_token and returns an error if it is invalid.
func (o *OIDCTokenVerifier) Verify(ctx context.Context, rawToken string) error {
	_, err := o.v.Verify(ctx, rawToken)
	return err
}
