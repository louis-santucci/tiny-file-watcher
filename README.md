# tiny-file-watcher

A macOS file-watching service that monitors directories for new files, tracks them in a SQLite database, and redirects/copies them to target locations. Built with gRPC, it consists of a long-running server (`tfws`) and a CLI client (`tfw`).

## Requirements

- Go 1.25+
- macOS (LaunchAgent integration is macOS-specific; core functionality is cross-platform)
- `protoc` with `protoc-gen-go` and `protoc-gen-go-grpc` (only needed to modify the proto schema)

## Configuration

Both `tfws` and `tfw` read from `~/.tfw/tfw.yml`. Create this file before starting the server:

```yaml
grpc.address: "localhost:50051"
debug-ui.address: "localhost:8080"
debug-ui.enabled: "true"
db.name: "tfw.db"
```

The SQLite database is stored in `~/.tfw/<db.name>`.

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
make install-service    # install and start
make uninstall-service  # stop and remove
make enable-service     # load (start)
make disable-service    # unload (stop)
```

## CLI Reference

### Watchers

Watchers monitor a source directory for new and removed files.

```bash
tfw watcher list
tfw watcher get <name>
tfw watcher create <name> --path /path/to/watch
tfw watcher update <name> [--name new-name] [--path new-path]
tfw watcher delete <name>
tfw watcher toggle <name>   # enable/disable
```

Alias: `tfw w`

### Redirections

A redirection defines where a watcher's detected files are copied to when flushed.

```bash
tfw redirection get <watcher-name>
tfw redirection create <watcher-name> --target /path/to/target [--auto-flush]
tfw redirection update <watcher-name> [--target new-path] [--auto-flush true|false]
tfw redirection delete <watcher-name>
```

Alias: `tfw r`

### Flush

Copies pending (unflushed) files to the watcher's redirection target.

```bash
tfw flush pending <watcher-name> [-p]   # list pending files; -p shows full path
tfw flush run <watcher-name>            # copy pending files to target
```

### Filters

Filters control which files a watcher picks up. No filters means accept everything. Exclude rules always win over include rules. If any include rules exist, a file must match at least one.

```bash
tfw filter list [watcher-name]
tfw filter add <watcher-name> --type <include|exclude> --match <extension|name|glob> --pattern <value>
tfw filter delete <filter-id>
```

Examples:
```bash
# Only accept .mp3 and .wav files
tfw filter add my-watcher --type include --match extension --pattern .mp3
tfw filter add my-watcher --type include --match extension --pattern .wav

# Exclude temp files and macOS metadata
tfw filter add my-watcher --type exclude --match glob --pattern "*.tmp"
tfw filter add my-watcher --type exclude --match name --pattern ".DS_Store"
```

## Typical Workflow

```bash
# 1. Create a watcher for a source directory
tfw watcher create downloads --path ~/Downloads

# 2. Set where detected files should be copied
tfw redirection create downloads --target ~/Sorted/Downloads --auto-flush

# 3. Check what has been detected
tfw flush pending downloads

# 4. Manually flush (or rely on auto-flush)
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

### Project Structure

```
grpc/filewatcher.proto   # API definition — edit this to change the schema
gen/grpc/               # generated protobuf stubs (do not edit manually)
server/                 # gRPC server (tfws)
  app.go                # component wiring
  database/             # SQLite persistence layer + schema.sql
  watcher/              # fsnotify goroutine manager + FileWatcherService
  flush/                # FileFlushService
  redirection/          # FileRedirectionService
  filter/               # WatcherFilterService
client/cmd/             # Cobra CLI commands (tfw)
launchd/                # macOS LaunchAgent plist template
```
