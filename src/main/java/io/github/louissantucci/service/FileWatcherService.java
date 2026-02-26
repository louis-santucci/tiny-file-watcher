package io.github.louissantucci.service;

import io.github.louissantucci.model.FileWatcher;
import io.github.louissantucci.persistence.FileWatcherEntity;
import io.github.louissantucci.persistence.repository.FileWatcherRepository;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.persistence.EntityNotFoundException;
import jakarta.transaction.Transactional;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;

import java.io.IOException;
import java.nio.file.*;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;

@ApplicationScoped
@Slf4j
@RequiredArgsConstructor
public class FileWatcherService {
    private final FileService fileService;
    private final FileWatcherRepository fileWatcherRepository;
    /**
     * Tracks all active WatchService instances so they can be closed on shutdown.
     */
    private final Map<Long, WatchService> activeWatchers = new ConcurrentHashMap<>();

    /**
     * Register a WatchService instance so it can be stopped later.
     */
    public void register(final long id, final WatchService watchService) {
        activeWatchers.put(id, watchService);
    }

    /**
     * Persists a detected file within a transaction, skipping duplicates.
     * Must live on this CDI bean so the proxy/transaction context works correctly.
     */
    @Transactional
    public void persistFile(long watcherId, Path filePath, boolean flushed) {
        FileWatcherEntity entity = fileWatcherRepository.findById(watcherId);
        fileService.persistFile(entity, filePath, flushed);
    }

    /**
     * Closes all active WatchService instances, which unblocks any threads waiting on take().
     */
    public void stopAll() {
        activeWatchers.keySet().forEach(this::stopWatcher);
    }

    public void stopWatcher(long id) {
        WatchService watchService = activeWatchers.remove(id);
        if (watchService != null) {
            try {
                watchService.close();
            } catch (IOException e) {
                log.warn("Error closing WatchService for watcher id {}: {}", id, e.getMessage());
            }
        } else {
            log.warn("No active WatchService found for watcher id {}.", id);
        }
    }

    public FileWatcher getWatcher(long id) {
        var watcherEntity = fileWatcherRepository.findByIdOptional(id);
        WatchService watchService = activeWatchers.get(id);
        if (watchService == null) {
            throw new EntityNotFoundException("File watcher with id " + id + " not found");
        }
        return watcherEntity.map(entity -> new FileWatcher(
                entity.getId(),
                Path.of(entity.getSource()),
                Path.of(entity.getDestination()),
                watchService)).orElseThrow(EntityNotFoundException::new);
    }

    public Set<FileWatcherEntity> getWatcherEntities() {
        return fileWatcherRepository.listAll().stream().collect(HashSet::new, Set::add, Set::addAll);
    }

    public Set<FileWatcher> getWatchers() {
        Set<FileWatcher> watchers = new HashSet<>();
        var watcherEntities = fileWatcherRepository.listAll();
        for (var entity : watcherEntities) {
            WatchService watchService = activeWatchers.get(entity.getId());
            if (watchService == null) {
                throw new EntityNotFoundException("File watcher with id " + entity.getId() + " not found");
            }
            watchers.add(new FileWatcher(
                    entity.getId(),
                    Path.of(entity.getSource()),
                    Path.of(entity.getDestination()),
                    watchService));
        }
        return watchers;
    }

    public void runWatcher(FileWatcher fileWatcher) throws IOException {
        long id = fileWatcher.id();
        WatchService watchService = fileWatcher.watchService();
        register(id, watchService);
        fileWatcher.source().register(watchService, StandardWatchEventKinds.ENTRY_CREATE);

        log.info("Virtual thread [{}] now watching '{}'.",
                Thread.currentThread().getName(), fileWatcher.source());

        while (!Thread.currentThread().isInterrupted()) {
            WatchKey key;
            try {
                key = watchService.take(); // blocks until an event or the WatchService is closed
            } catch (InterruptedException | ClosedWatchServiceException _) {
                Thread.currentThread().interrupt();
                break;
            }

            for (WatchEvent<?> watchEvent : key.pollEvents()) {
                if (watchEvent.kind() == StandardWatchEventKinds.OVERFLOW) {
                    continue;
                }

                @SuppressWarnings("unchecked")
                WatchEvent<Path> pathEvent = (WatchEvent<Path>) watchEvent;
                Path detectedFile = fileWatcher.source().resolve(pathEvent.context());

                try {
                    persistFile(fileWatcher.id(), detectedFile, false);
                    log.info("FileWatcher [id={}] detected and persisted: '{}'.",
                            fileWatcher.id(), detectedFile);
                } catch (Exception e) {
                    log.error("FileWatcher [id={}] failed to persist file '{}': {}",
                            fileWatcher.id(), detectedFile, e.getMessage());
                }
            }

            boolean valid = key.reset();
            if (!valid) {
                log.warn("FileWatcher [id={}] watch key invalidated — stopping watcher for '{}'.",
                        fileWatcher.id(), fileWatcher.source());
                break;
            }
        }


        log.info("Virtual thread [{}] exiting.", Thread.currentThread().getName());
    }
}
