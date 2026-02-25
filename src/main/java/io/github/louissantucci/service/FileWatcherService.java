package io.github.louissantucci.service;

import io.github.louissantucci.persistence.repository.FileRepository;
import io.github.louissantucci.persistence.repository.FileWatcherRepository;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;

@ApplicationScoped
public class FileWatcherService {
    private final FileWatcherRepository fileWatcherRepository;
    private final FileRepository fileRepository;

    @Inject
    public FileWatcherService(FileWatcherRepository fileWatcherRepository,
                              FileRepository fileRepository) {
        this.fileWatcherRepository = fileWatcherRepository;
        this.fileRepository = fileRepository;
    }

}
