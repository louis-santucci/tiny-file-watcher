# Plan: REST Migration — Typed HTTP Client, DTOs, 204 Delete, `--url` flag

Full replacement of gRPC with a REST/JSON layer. The server exposes four JAX-RS endpoints. The client wraps `java.net.http.HttpClient` in a typed `FileWatcherHttpClient` with one method per endpoint, each returning a strongly-typed DTO. A root `--url` picocli option replaces the hardcoded host/port.

---

## Steps

### 1. Server `pom.xml`
In `tiny-file-watcher-server/pom.xml`: remove `quarkus-grpc`, add `quarkus-rest` and `quarkus-rest-jackson`.

### 2. Server DTOs
Create three records in a new `io.github.louissantucci.rest.dto` package:
- `FileWatcherDto(long id, String source, String destination, boolean enabled)`
- `FileWatcherRequest(String source, String destination)` — used for `POST` body
- `ApiResponse<T>(String status, String errorMessage, T data)` — wraps `GET` responses; `DELETE` returns `204` with no body

### 3. Server REST resource
Create `io.github.louissantucci.rest.FileWatcherResource` (`@Path("/api/watchers")`), delegating to `FileWatcherService`:
- `GET /api/watchers?active=` → `ApiResponse<List<FileWatcherDto>>`
- `GET /api/watchers/{id}` → `ApiResponse<FileWatcherDto>`
- `POST /api/watchers` → `201 Created` + `ApiResponse<FileWatcherDto>`
- `DELETE /api/watchers/{id}` → `204 No Content` (error cases return `404`)

### 4. Server cleanup
- Delete `FileWatcherGrpcService.java`
- Delete `tiny-file-watcher-server/src/main/proto/tinyfilewatcher.proto`
- Remove `quarkus.grpc.server.use-separate-server=false` from `application.properties`

### 5. Client `pom.xml`
In `tiny-file-watcher-client/pom.xml`:
- Remove all `io.grpc.*`, `com.google.protobuf`, `javax.annotation` dependencies
- Remove the `protoc-jar-maven-plugin`
- Add `jackson-databind` and `jackson-datatype-jdk8`
- Delete `tiny-file-watcher-client/src/main/proto/`

### 6. Client DTOs
Mirror the same three records in `io.github.louissantucci.client.dto` (same fields as server step 2, no shared module):
- `FileWatcherDto(long id, String source, String destination, boolean enabled)`
- `FileWatcherRequest(String source, String destination)`
- `ApiResponse<T>(String status, String errorMessage, T data)`

### 7. Client `FileWatcherHttpClient`
Replace `FileWatcherGrpcClient` with `io.github.louissantucci.client.http.FileWatcherHttpClient(String baseUrl)`, backed by `java.net.http.HttpClient` + Jackson `ObjectMapper`:
- `ApiResponse<List<FileWatcherDto>> listWatchers(Boolean active)` — `GET /api/watchers?active=`
- `ApiResponse<FileWatcherDto> getWatcher(long id)` — `GET /api/watchers/{id}`
- `ApiResponse<FileWatcherDto> createWatcher(String source, String destination)` — `POST /api/watchers`
- `void deleteWatcher(long id)` — `DELETE /api/watchers/{id}`, asserts `204`, throws on non-2xx

> **Note:** Deserialising `ApiResponse<List<FileWatcherDto>>` requires `new TypeReference<ApiResponse<List<FileWatcherDto>>>(){}` to preserve generics at runtime.

### 8. Client commands + `--url` flag
- Add `@CommandLine.Option(names = {"--url"}, defaultValue = "http://localhost:8080")` field on `TinyFileWatcher` (root command)
- Access it from sub-commands via picocli's `@ParentCommand` injection
- Update `ListWatchersCommand` to instantiate `new FileWatcherHttpClient(parent.url)` and call `listWatchers(active)`
- Update `GetSingleWatcherCommand` to instantiate `new FileWatcherHttpClient(parent.url)` and call `getWatcher(watcherId)`
- Delete `FileWatcherGrpcClient.java` and the `grpc/` package entirely

---

## Further Considerations

1. **`@ParentCommand` injection** — picocli's `@ParentCommand` field on each sub-command is the cleanest way to access the root `--url` without a static field or manual wiring.
2. **Jackson `TypeReference`** — deserialising `ApiResponse<List<FileWatcherDto>>` requires a `new TypeReference<ApiResponse<List<FileWatcherDto>>>(){}` to preserve generics at runtime; worth noting this as a non-obvious implementation detail.
3. **Error propagation** — for `GET /{id}` when the server returns `404`, `FileWatcherHttpClient` should throw a dedicated `WatcherNotFoundException` so the command can print a clean message rather than catching a raw HTTP status.
4. **Default config** — the `--url` default value `http://localhost:8080` is a placeholder; a proper config file mechanism (e.g. `~/.tfw/config.properties`) can be layered on top later without changing the client architecture.

