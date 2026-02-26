package io.github.louissantucci.config;

import io.github.louissantucci.model.FileWatcher;
import io.github.louissantucci.persistence.FileWatcherEntity;
import io.github.louissantucci.service.FileWatcherService;
import io.quarkus.runtime.ShutdownEvent;
import io.quarkus.runtime.Startup;
import io.quarkus.runtime.StartupEvent;
import jakarta.enterprise.event.Observes;
import jakarta.transaction.Transactional;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;

import java.io.IOException;
import java.nio.file.FileSystems;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.WatchService;

@Startup
@Slf4j
@RequiredArgsConstructor
public class FileWatcherStartup {
    private final FileWatcherService watcherService;

    @Transactional
    void onStart(@Observes StartupEvent event) throws IOException {
        var watchers = watcherService.getWatcherEntities();
        log.info("Starting file watchers — found {} watcher(s).", watchers.size());

        for (FileWatcherEntity entity : watchers) {
            Path sourcePath = Path.of(entity.getSource());

            if (!Files.isDirectory(sourcePath)) {
                log.warn("FileWatcher [id={}] skipped: source path '{}' does not exist or is not a directory.",
                        entity.getId(), entity.getSource());
                continue;
            }

            // Mark all existing files as already flushed before starting the watcher
            try (var stream = Files.walk(sourcePath)) {
                stream.filter(Files::isRegularFile)
                        .forEach(file -> watcherService.persistFile(entity.getId(), file, true));
            }
            log.info("FileWatcher [id={}] pre-marked existing files in '{}' as flushed.", entity.getId(), entity.getSource());

            long watcherId = entity.getId();
            String watcherSource = entity.getSource();
            WatchService watchService = FileSystems.getDefault().newWatchService();
            FileWatcher fileWatcher = new FileWatcher(watcherId, sourcePath, Path.of(entity.getDestination()), watchService);

            Thread.ofVirtual()
                    .name("fw-" + watcherId)
                    .start(() -> {
                        try {
                            watcherService.runWatcher(fileWatcher);
                        } catch (IOException e) {
                            throw new RuntimeException(e);
                        }
                    });

            log.info("FileWatcher [id={}] started watching '{}'.", watcherId, watcherSource);
        }
    }

    void onStop(@Observes ShutdownEvent event) {
        log.info("Shutting down all file watchers...");
        watcherService.stopAll();
    }


}
