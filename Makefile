
.PHONY: clean install test \
        run-server run-client \
        build-server-native build-client-native

# ── full build ────────────────────────────────────────────────────────────────
install:
	@mvn clean install -DskipTests

clean:
	@mvn clean

test:
	@mvn test -pl tiny-file-watcher-server

# ── dev mode ──────────────────────────────────────────────────────────────────
run-server:
	@mvn quarkus:dev -pl tiny-file-watcher-server

run-client:
	@mvn quarkus:dev -pl tiny-file-watcher-client

# ── native builds ─────────────────────────────────────────────────────────────
build-server-native:
	@mvn install -DskipTests -Pnative -pl tiny-file-watcher-proto,tiny-file-watcher-server

build-client-native:
	@mvn install -DskipTests -Pnative -pl tiny-file-watcher-proto,tiny-file-watcher-client
