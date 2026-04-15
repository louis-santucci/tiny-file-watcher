# Plan: `TFWS_CONFIG_PATH` env override + Dockerfile

## Context

**tiny-file-watcher** server (`tfws`) currently resolves all runtime paths
(`tfws.yml`, `tfw.db`, …) by calling `internal.Dir()`, which returns
`~/.tfw/`. In a container there is no meaningful home directory, so a single
env variable `TFWS_CONFIG_PATH` must override that base directory. When the
variable is absent the existing `UserHomeDir + .tfw` mechanism is kept intact.

The user will supply **one** bind-mount in their docker-compose file pointing to
a host directory that contains both `tfws.yml` and `tfw.db`. No extra env vars
are needed beyond `TFWS_CONFIG_PATH`.

---

## Step 1 — Modify `Dir()` in `internal/tfw_path.go`

Add an `os.Getenv("TFWS_CONFIG_PATH")` check at the very top of `Dir()`.
If the variable is non-empty, return it immediately, bypassing
`os.UserHomeDir()`. No other functions need to change — `ServerConfigPath()`,
`DatabasePath()`, `TokensPath()`, `MachinePath()`, and `ClientConfigPath()` all
flow through `Dir()` and inherit the override automatically.

```go
func Dir() (string, error) {
    if envDir := os.Getenv("TFWS_CONFIG_PATH"); envDir != "" {
        return envDir, nil
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return "", fmt.Errorf("could not determine home directory: %w", err)
    }
    return filepath.Join(home, DirName), nil
}
```

---

## Step 2 — Create `Dockerfile` at the project root

Two-stage build:

### Builder stage
- Base image: `dhi.io/golang/1.26.1-debian13-dev`
- Set `CGO_ENABLED=0` and `GOOS=linux` for a fully static binary
  (`modernc.org/sqlite` is a pure-Go SQLite transpilation — no CGo required).
- Copy the full source tree (including the already-committed `gen/grpc/` files).
- Run `go build -o /tfws ./server` directly — no `make generate` needed,
  avoiding the need for `protoc` in the builder image.

### Runtime stage
- Base image: `dhi.io/debian-base/trixie-debian13`
- Copy only `/tfws` from the builder stage.
- Set `ENV TFWS_CONFIG_PATH=/tfw` as the container-default base directory.
- Declare `VOLUME /tfw` so the mount point is self-documented in the image.
- Set `ENTRYPOINT ["/tfws"]`.
- **No `EXPOSE` directive** — the gRPC address (and optional web / debug-ui
  addresses) are fully driven by the `tfws.yml` config mounted at
  `$TFWS_CONFIG_PATH/tfws.yml`.

### Expected docker-compose snippet

```yaml
services:
  tfws:
    image: tfws:latest
    environment:
      TFWS_CONFIG_PATH: /tfw
    volumes:
      - ./my-tfw-dir:/tfw   # must contain tfws.yml and will store tfw.db
```

---

## Further Considerations

1. **`CGO_ENABLED=0` compatibility**: `modernc.org/sqlite` is a pure-Go SQLite
   transpilation and is CGo-free — confirmed safe for a fully static binary. The
   runtime image needs no extra `libc` or `libsqlite3`.

2. **Volume convention**: with `TFWS_CONFIG_PATH=/tfw`, the compose file mounts
   a host directory to `/tfw` containing both `tfws.yml` and `tfw.db` — no
   extra env vars needed beyond the single `TFWS_CONFIG_PATH`.

3. **`database.path` config key**: `app.go` already supports a `database.path`
   config key that overrides `internal.DatabasePath()`. With `TFWS_CONFIG_PATH`
   set, both the config file and the database resolve to the same mounted
   directory automatically — no extra config key needed.

4. **`go build` vs `make build-server`**: using `go build` directly in the
   Dockerfile avoids needing `protoc` installed in the builder image. This is
   safe as long as `gen/grpc/` is committed (which it is). If `make` is
   preferred in the future, `protoc` would need to be installed separately in
   the builder.

