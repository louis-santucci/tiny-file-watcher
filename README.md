# tiny-file-watcher

A file-watching automation service that monitors directories (local or remote via SSH/SFTP) for new and removed files, tracks them in a SQLite database, and copies them to configured target locations. Built with gRPC, it consists of a long-running server (`tfws`) and a CLI client (`tfw`).

## Architecture

```
tfw (CLI client) ──gRPC──▶ tfws (server)
                               │
                               ├── SQLite (state)
                               ├── SSH/SFTP (remote filesystem access)
                               └── Web UI (optional)
```

**`tfws`** is the server process. It exposes four gRPC services:
- `FileWatcherService` — CRUD for watchers, sync (unary + streaming)
- `FileFlushService` — copy pending files to their target
- `FileRedirectionService` — CRUD for per-watcher copy targets
- `MachineService` — register and manage machines

**`tfw`** is the CLI client. It dials `tfws` over gRPC to manage every resource.

## Requirements

- Go 1.25+
- macOS or Linux
- `protoc` with `protoc-gen-go` and `protoc-gen-go-grpc` (only needed to modify the proto schema)

## Configuration

All config and state files live under `~/.tfw/`.

### Server (`~/.tfw/tfws.yml`)

```yaml
# Required
grpc.address: "localhost:50051"
ssh.private_keys_path: "~/.ssh"
ssh.known_hosts_path: "~/.ssh/known_hosts"

# Optional: browser-based gRPC debug UI
debug-ui.enabled: "true"
debug-ui.address: "localhost:8080"

# Optional: web UI (requires OIDC when enabled)
web.enabled: "true"
web.address: "localhost:8081"
oidc.enabled: "true"
oidc.issuer: "https://your-oidc-provider"
oidc.client-id: "your-client-id"
oidc.client-secret: "your-client-secret"
oidc.redirect-uri: "http://localhost:8081/auth/callback"

# Optional
log.level: "info"
database.path: "~/.tfw/tfw.db"
```

### Client (`~/.tfw/tfw.yml`)

```yaml
# Required
grpc.address: "localhost:50051"

# Optional: OIDC authentication
oidc.enabled: "true"
oidc.issuer: "https://your-oidc-provider"
oidc.device-client-id: "your-device-client-id"
```

The SQLite database defaults to `~/.tfw/tfw.db`.

## Installation

```bash
# Build and install both binaries to $GOPATH/bin
make install

# Or build locally without installing
make build
```

## Running

**Start the server:**
```bash
tfws
```

**Use the CLI (server must be running):**
```bash
tfw <command>
```

### macOS LaunchAgent (run server at login)

```bash
make install-service    # install, register, and start
make uninstall-service  # stop and remove
make enable-service     # load (start)
make disable-service    # unload (stop)
```

### Linux systemd user service

```bash
make install-service-linux    # install, register, and start
make uninstall-service-linux  # stop and remove
make enable-service-linux     # enable and start
make disable-service-linux    # disable and stop
```

## CLI Reference

### Authentication

OIDC device-flow login (required when `oidc.enabled=true` on the server):

```bash
tfw login
tfw logout
```

Tokens are stored at `~/.tfw/tokens.json`.

### Machines

A machine represents a host (local or remote) that `tfws` can watch over SSH. Watchers are scoped to a machine.

```bash
tfw machine create <name> --ip <ip> --ssh-user <user> --ssh-key <path/to/key> [--ssh-port 22]
tfw machine list
tfw machine delete <name>
```

The machine token is saved locally to `~/.tfw/machine.json` and used to authenticate sync calls.

Alias: `tfw m`

### Watchers

A watcher monitors a source directory on a registered machine for new and removed files.

```bash
tfw watcher list [--machine <machine-name>]
tfw watcher get <name>
tfw watcher create <name> --path /path/to/watch --machine <machine-name>
tfw watcher update <name> [--name new-name] [--path new-path]
tfw watcher delete <name>
tfw watcher files <name>    # list all tracked files for this watcher
```

Place a `.tfwignore` file in the watched directory to exclude paths using gitignore-style patterns.

Alias: `tfw w`

### Sync

Sync walks the watcher's source directory (over SSH/SFTP) and reconciles its contents with the database — adding newly detected files and removing entries for deleted ones.

```bash
tfw sync <watcher-name>           # unary sync
tfw sync <watcher-name> --stream  # streaming sync with live log output
```

The machine token from `~/.tfw/machine.json` is sent with each sync request for ownership validation.

### Redirections

A redirection defines where a watcher's detected files are copied when flushed.

```bash
tfw redirection get <watcher-name>
tfw redirection create <watcher-name> --target /path/to/target [--auto-flush]
tfw redirection update <watcher-name> [--target new-path] [--auto-flush true|false]
tfw redirection delete <watcher-name>
```

When `--auto-flush` is set, files are copied automatically at sync time.

Alias: `tfw r`

### Flush

Copies pending (unflushed) tracked files to the watcher's redirection target.

```bash
tfw flush pending <watcher-name> [-p]   # list pending files; -p shows full path
tfw flush run <watcher-name>            # copy pending files to target
```

## Typical Workflow

```bash
# 1. Register this machine with the server
tfw machine create my-mac --ip 192.168.1.10 --ssh-user alice --ssh-key ~/.ssh/id_rsa

# 2. Create a watcher for a source directory
tfw watcher create downloads --path ~/Downloads --machine my-mac

# 3. Run an initial sync to populate the database
tfw sync downloads

# 4. Set where detected files should be copied
tfw redirection create downloads --target ~/Sorted/Downloads --auto-flush

# 5. Check what has been detected
tfw flush pending downloads

# 6. Manually flush (or rely on auto-flush on next sync)
tfw flush run downloads
```

## Development

```bash
make build          # compile tfw + tfws
make test           # run all tests (race detector + integration tag)
make lint           # run golangci-lint
make generate       # regenerate Go code from grpc/filewatcher.proto
make install-tools  # install protoc plugins and golangci-lint
make clean          # remove built binaries and generated proto files
```

Run a single test:
```bash
go test -run TestName ./server/watcher/...
```

Integration tests require the `-tags integration` flag (included in `make test`).

### Project Structure

```
grpc/filewatcher.proto      # gRPC API definition — single source of truth
gen/grpc/                   # generated protobuf stubs (do not edit manually)

server/                     # tfws server binary
  main.go                   # entrypoint
  app.go                    # component wiring (config, DB, gRPC, web)
  config/                   # config validation (grpc, ssh, web, oidc, debug-ui)
  database/                 # SQLite persistence layer + schema.sql
  watcher/                  # FileWatcherService, SyncJob, .tfwignore support
  flush/                    # FileFlushService
  redirection/              # FileRedirectionService
  machine/                  # MachineService
  interceptor/              # gRPC auth (Bearer token) and logging interceptors
  web/                      # optional HTML web UI with OIDC auth
  test/                     # integration tests

client/                     # tfw CLI binary
  main.go                   # entrypoint
  cmd/                      # Cobra commands (watcher, sync, flush, redirection, machine, login)
  auth/                     # OIDC token storage and gRPC credentials
  machine/                  # local machine state (name + token)
  config/                   # client config loader

internal/                   # shared library code
  set.go                    # generic Set[T] data structure
  tfw_config.go             # config loader initializer
  tfw_path.go               # ~/.tfw/* path constants

launchd/                    # macOS LaunchAgent plist template
systemd/                    # Linux systemd user service unit template
```
