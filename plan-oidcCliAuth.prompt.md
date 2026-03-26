# Plan: Add OIDC Auth to gRPC CLI (using `id_token`)

Reuse the existing `go-oidc/v3` and `oauth2` dependencies to add device-flow login to `tfw` and `id_token` validation to the `tfws` gRPC server. The CLI stores the `id_token` locally and sends it as a Bearer token on every call; the server validates it with the same `IDTokenVerifier` already used by the web layer.

## Steps

1. **Add `server/interceptor/token_verifier.go`** — define a `TokenVerifier` interface (`Verify(ctx, rawToken) error`), a `noopVerifier` (OIDC disabled), and an `OIDCTokenVerifier` that wraps `*gooidc.IDTokenVerifier` from `github.com/coreos/go-oidc/v3` — reusing the same provider discovery already done in `server/web/auth.go`.

2. **Add `server/interceptor/grpc_auth.go`** — a `NewAuthInterceptors(v TokenVerifier)` function returning both a `grpc.UnaryServerInterceptor` and a `grpc.StreamServerInterceptor`; each extracts `authorization: Bearer <token>` from incoming gRPC metadata and calls `v.Verify`, returning `codes.Unauthenticated` on failure. Follow the style of `server/interceptor/grpc_logger.go`.

3. **Modify `server/app.go`** — after `oidcCfgFromConfig`, conditionally build an `OIDCTokenVerifier` (or `noopVerifier`), then upgrade `grpc.NewServer` to use `grpc.ChainUnaryInterceptor(logging, auth)` and add `grpc.ChainStreamInterceptor(streamAuth)`.

4. **Add `client/auth/tokens.go`** — persist/load/clear a `TokenSet{IDToken, RefreshToken, Expiry}` as JSON at `~/.tfw/tokens.json` (`0600`). The CLI saves the `id_token` string from `token.Extra("id_token")` returned by the device flow exchange.

5. **Add `client/auth/grpc_creds.go`** — implement `credentials.PerRPCCredentials`: load the stored `id_token` and return `authorization: Bearer <id_token>`; if the token is expired and a `refresh_token` is present, re-exchange via `oauth2` and extract the new `id_token` from `Extra("id_token")`; fail fast with `"not logged in: run 'tfw login' first"` if no token file exists. `RequireTransportSecurity()` returns `false` because TLS is terminated at the reverse proxy — the CLI-to-proxy leg is already encrypted; the proxy-to-`tfws` leg is plaintext on a trusted internal/loopback network.

6. **Add `client/cmd/login.go`** — `tfw login`: reads `oidc.issuer` and `oidc.device-client-id` from `~/.tfw/tfw.yml`, runs the device authorization grant (`DeviceAuth` → print URL → `DeviceAccessToken`), extracts `id_token` via `token.Extra("id_token")`, saves the `TokenSet`; `tfw logout` calls `auth.ClearTokens()`.

7. **Modify `client/cmd/root.go`** — in `dial()`, read `oidc.enabled` and `server.addr` from config. When `oidc.enabled` is true, connect with `credentials.NewTLS(&tls.Config{})` (system CA pool, no custom cert required) so the CLI speaks TLS to the reverse proxy; also append `grpc.WithPerRPCCredentials(auth.NewTokenCredentials(...))`, failing early with a clear message if no token file is found. When `oidc.enabled` is false (local dev), fall back to `insecure.NewCredentials()` as today.

## PocketID Setup

The web UI client is a **confidential** client (has a `client-secret`, uses authorization code flow). The device flow requires a **public** client (no secret, no redirect URI). A single PocketID OAuth2 application cannot be both, so a **second application must be created**.

Create a second OAuth2 application in PocketID for the CLI:

| Setting | Value |
|---|---|
| **Type** | Public |
| **Grant types** | Device Authorization Grant + Refresh Token |
| **Scopes** | `openid profile email` |
| **Redirect URI** | *(leave empty)* |
| **Client Secret** | *(none)* |

Then in `~/.tfw/tfw.yml`, add a dedicated key alongside the existing web ones:

```yaml
oidc:
  enabled: true
  issuer: https://your-pocketid-instance
  client-id: <web-ui-client-id>             # existing, for tfws web UI
  client-secret: <web-ui-client-secret>     # existing, for tfws web UI
  redirect-uri: http://localhost:...        # existing, for tfws web UI
  device-client-id: <new-public-client-id>  # new, for tfw CLI device flow
```

## Further Considerations

1. **`id_token` expiry** — `id_token`s are typically short-lived (1–5 min on PocketID). The refresh step in `grpc_creds.go` re-exchanges the refresh token and extracts the new `id_token` from the response; confirm PocketID returns a new `id_token` on refresh (most providers do, but it is not mandated by the spec).

2. **TLS strategy (Option A — proxy terminates TLS)** — `tfws` itself does not need a `.crt`/`.key` and continues to listen on plain HTTP/2 (`h2c`). The reverse proxy (Caddy or nginx) owns the TLS certificate (e.g. via Let's Encrypt) and forwards traffic to `tfws` over the internal/loopback network in plaintext. Because the gRPC library has no visibility into the proxy's TLS, `PerRPCCredentials.RequireTransportSecurity()` returns `false` on the server's transport level — the Bearer token is still protected by the proxy's TLS on the public leg. The `tfws` gRPC server config does **not** need `grpc.Creds(...)` or `credentials.NewServerTLSFromFile`.

3. **Reverse proxy config** — the proxy must support HTTP/2 (gRPC requires it). With **Caddy**, declare the upstream as `h2c` (e.g. `reverse_proxy h2c://localhost:<port>`); with **nginx**, use `grpc_pass grpc://localhost:<port>`. The proxy handles HTTPS termination and forwards the `authorization` header untouched to `tfws`.

4. **`grpc.ChainStreamInterceptor`** — currently the server only chains unary interceptors; a stream auth interceptor should also be added for completeness, following the same pattern.

