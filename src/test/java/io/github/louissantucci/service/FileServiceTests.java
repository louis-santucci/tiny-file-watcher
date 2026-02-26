package io.github.louissantucci.service;

import static org.mockito.Mockito.*;

import io.github.louissantucci.persistence.FileEntity;
import io.github.louissantucci.persistence.FileWatcherEntity;
import io.github.louissantucci.persistence.repository.FileRepository;
import io.quarkus.hibernate.orm.panache.PanacheQuery;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class FileServiceTests {
  private final FileRepository fileRepository = mock(FileRepository.class, RETURNS_DEEP_STUBS);

  private FileService fileService;

  @BeforeEach
  void setUp() {
    reset(fileRepository);
    fileService = spy(new FileService(fileRepository));
  }

  @Test
  void testShouldNotPersistFileWhenUnflushedFileExistsInDb() {
    FileWatcherEntity fileWatcherEntity = mock(FileWatcherEntity.class);
    when(fileWatcherEntity.getId()).thenReturn(1L);
    FileEntity fileEntity =
        FileEntity.builder()
            .id(5L)
            .path("/tmp/test.mp3")
            .filename("test.mp3")
            .isFlushed(false)
            .fileWatcher(fileWatcherEntity)
            .build();
    PanacheQuery<FileEntity> panacheQuery =
        mock(io.quarkus.hibernate.orm.panache.PanacheQuery.class);
    when(fileRepository.find(anyString(), anyLong(), anyString())).thenReturn(panacheQuery);
    when(panacheQuery.firstResultOptional()).thenReturn(java.util.Optional.of(fileEntity));

    fileService.persistFile(fileWatcherEntity, java.nio.file.Path.of("/tmp/test.mp3"), true);
    verify(fileRepository, never()).persist(any(FileEntity.class));
  }

  @Test
  void testShouldNotPersistFileWhenFlushedFileExistsInDb() {
    FileWatcherEntity fileWatcherEntity = mock(FileWatcherEntity.class);
    when(fileWatcherEntity.getId()).thenReturn(1L);
    FileEntity fileEntity =
        FileEntity.builder()
            .id(5L)
            .path("/tmp/test.mp3")
            .filename("test.mp3")
            .isFlushed(true)
            .fileWatcher(fileWatcherEntity)
            .build();
    PanacheQuery<FileEntity> panacheQuery =
        mock(io.quarkus.hibernate.orm.panache.PanacheQuery.class);
    when(fileRepository.find(anyString(), anyLong(), anyString())).thenReturn(panacheQuery);
    when(panacheQuery.firstResultOptional()).thenReturn(java.util.Optional.of(fileEntity));

    fileService.persistFile(fileWatcherEntity, java.nio.file.Path.of("/tmp/test.mp3"), true);
    verify(fileRepository, never()).persist(any(FileEntity.class));
  }

  @Test
  void testShouldPersistFileWhenFileDoesNotExistInDb() {
    FileWatcherEntity fileWatcherEntity = mock(FileWatcherEntity.class);
    when(fileWatcherEntity.getId()).thenReturn(1L);
    PanacheQuery<FileEntity> panacheQuery =
        mock(io.quarkus.hibernate.orm.panache.PanacheQuery.class);
    when(fileRepository.find(anyString(), anyLong(), anyString())).thenReturn(panacheQuery);
    when(panacheQuery.firstResultOptional()).thenReturn(java.util.Optional.empty());

    fileService.persistFile(fileWatcherEntity, java.nio.file.Path.of("/tmp/test.mp3"), true);
    verify(fileRepository, times(1)).persist(any(FileEntity.class));
  }
}
