package io.github.louissantucci.service;

import io.github.louissantucci.persistence.FileEntity;
import io.github.louissantucci.persistence.FileWatcherEntity;
import io.github.louissantucci.persistence.repository.FileRepository;
import jakarta.enterprise.context.ApplicationScoped;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;

import java.nio.file.Path;

@ApplicationScoped
@RequiredArgsConstructor
@Slf4j
public class FileService {
    private final FileRepository fileRepository;

    public void persistFile(FileWatcherEntity fileWatcherEntity, Path filePath, boolean flushed) {
        boolean exists = fileRepository
                .find("fileWatcher.id = ?1 and filename = ?2",
                        fileWatcherEntity.getId(),
                        filePath.getFileName().toString())
                .firstResultOptional()
                .isPresent();
        if (exists) {
            log.warn("File {} already exists", fileWatcherEntity.getId());
            return;
        }
        FileEntity fileEntity = FileEntity
                .builder().path(filePath.toString()).filename(filePath.getFileName().toString()).isFlushed(flushed).fileWatcher(fileWatcherEntity).build();
        fileRepository.persist(fileEntity);
    }
}
