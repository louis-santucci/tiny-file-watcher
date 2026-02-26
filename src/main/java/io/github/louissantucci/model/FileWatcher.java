package io.github.louissantucci.model;

import java.nio.file.Path;
import java.nio.file.WatchService;

public record FileWatcher(long id, Path source, Path destination, WatchService watchService) {}
