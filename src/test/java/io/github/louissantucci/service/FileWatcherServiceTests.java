package io.github.louissantucci.service;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

import io.github.louissantucci.model.FileWatcher;
import io.github.louissantucci.persistence.FileWatcherEntity;
import io.github.louissantucci.persistence.repository.FileWatcherRepository;
import io.quarkus.test.InjectMock;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import jakarta.persistence.EntityNotFoundException;
import java.io.IOException;
import java.nio.file.FileSystems;
import java.nio.file.Path;
import java.nio.file.WatchService;
import java.util.List;
import java.util.Optional;
import java.util.Set;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

@QuarkusTest
class FileWatcherServiceTests {

    @Inject
    FileWatcherService fileWatcherService;

    @InjectMock
    FileWatcherRepository fileWatcherRepository;

    @InjectMock
    FileService fileService;

    private FileWatcherEntity entityEnabled;
    private FileWatcherEntity entityDisabled;

    @BeforeEach
    void setUp() {
        entityEnabled = FileWatcherEntity.builder()
                .source("/tmp/source")
                .destination("/tmp/dest")
                .enabled(true)
                .build();
        setId(entityEnabled, 1L);

        entityDisabled = FileWatcherEntity.builder()
                .source("/tmp/source2")
                .destination("/tmp/dest2")
                .enabled(false)
                .build();
        setId(entityDisabled, 2L);

        // Reset activeWatchers between tests
        fileWatcherService.stopAll();
    }

    // Helper: set id via reflection since it's generated
    private void setId(FileWatcherEntity entity, long id) {
        try {
            var field = FileWatcherEntity.class.getDeclaredField("id");
            field.setAccessible(true);
            field.set(entity, id);
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }

    // --- isEnabled ---

    @Test
    void isEnabled_returnsTrueWhenEntityIsEnabled() {
        when(fileWatcherRepository.findByIdOptional(1L)).thenReturn(Optional.of(entityEnabled));

        assertTrue(fileWatcherService.isEnabled(1L));
    }

    @Test
    void isEnabled_returnsFalseWhenEntityIsDisabled() {
        when(fileWatcherRepository.findByIdOptional(2L)).thenReturn(Optional.of(entityDisabled));

        assertFalse(fileWatcherService.isEnabled(2L));
    }

    @Test
    void isEnabled_throwsEntityNotFoundExceptionWhenNotFound() {
        when(fileWatcherRepository.findByIdOptional(99L)).thenReturn(Optional.empty());

        assertThrows(EntityNotFoundException.class, () -> fileWatcherService.isEnabled(99L));
    }

    // --- register ---

    @Test
    void register_addsWatchServiceToActiveWatchers() throws IOException {
        // Build an entity whose id is 10 to match the registered key
        FileWatcherEntity entity10 = FileWatcherEntity.builder()
                .source("/tmp/source10")
                .destination("/tmp/dest10")
                .enabled(true)
                .build();
        setId(entity10, 10L);

        WatchService watchService = FileSystems.getDefault().newWatchService();
        fileWatcherService.register(10L, watchService);

        // Verify it is registered: getWatcher should find it (with entity mock too)
        when(fileWatcherRepository.findByIdOptional(10L)).thenReturn(Optional.of(entity10));

        FileWatcher watcher = fileWatcherService.getWatcher(10L);
        assertNotNull(watcher);
        assertEquals(10L, watcher.id());
        assertEquals(Path.of("/tmp/source10"), watcher.source());
        watchService.close();
    }

    // --- stopWatcher ---

    @Test
    void stopWatcher_closesAndRemovesActiveWatchService() throws IOException {
        WatchService watchService = mock(WatchService.class);
        fileWatcherService.register(1L, watchService);

        fileWatcherService.stopWatcher(1L);

        verify(watchService, times(1)).close();
    }

    @Test
    void stopWatcher_doesNotThrowWhenNoActiveWatcherExists() {
        // No watcher registered for id 999 — should simply log a warning
        assertDoesNotThrow(() -> fileWatcherService.stopWatcher(999L));
    }

    // --- stopAll ---

    @Test
    void stopAll_closesAllActiveWatchServices() throws IOException {
        WatchService ws1 = mock(WatchService.class);
        WatchService ws2 = mock(WatchService.class);
        fileWatcherService.register(1L, ws1);
        fileWatcherService.register(2L, ws2);

        fileWatcherService.stopAll();

        verify(ws1, times(1)).close();
        verify(ws2, times(1)).close();
    }

    // --- deleteWatcher ---

    @Test
    void deleteWatcher_deletesEntityAndStopsWatcher() throws IOException {
        WatchService watchService = mock(WatchService.class);
        fileWatcherService.register(1L, watchService);
        when(fileWatcherRepository.findByIdOptional(1L)).thenReturn(Optional.of(entityEnabled));

        fileWatcherService.deleteWatcher(1L);

        verify(fileWatcherRepository, times(1)).delete(entityEnabled);
        verify(watchService, times(1)).close();
    }

    @Test
    void deleteWatcher_throwsEntityNotFoundExceptionWhenEntityMissing() {
        when(fileWatcherRepository.findByIdOptional(99L)).thenReturn(Optional.empty());

        assertThrows(EntityNotFoundException.class, () -> fileWatcherService.deleteWatcher(99L));
        verify(fileWatcherRepository, never()).delete(any());
    }

    // --- getWatcher ---

    @Test
    void getWatcher_returnsFileWatcherWhenBothEntityAndWatchServiceExist() throws IOException {
        WatchService watchService = FileSystems.getDefault().newWatchService();
        fileWatcherService.register(1L, watchService);
        when(fileWatcherRepository.findByIdOptional(1L)).thenReturn(Optional.of(entityEnabled));

        FileWatcher result = fileWatcherService.getWatcher(1L);

        assertNotNull(result);
        assertEquals(1L, result.id());
        assertEquals(Path.of("/tmp/source"), result.source());
        assertEquals(Path.of("/tmp/dest"), result.destination());
        assertSame(watchService, result.watchService());

        watchService.close();
    }

    @Test
    void getWatcher_throwsEntityNotFoundExceptionWhenNoActiveWatchService() {
        when(fileWatcherRepository.findByIdOptional(1L)).thenReturn(Optional.of(entityEnabled));
        // No watcher registered

        assertThrows(EntityNotFoundException.class, () -> fileWatcherService.getWatcher(1L));
    }

    @Test
    void getWatcher_throwsEntityNotFoundExceptionWhenEntityMissing() throws IOException {
        WatchService watchService = FileSystems.getDefault().newWatchService();
        fileWatcherService.register(1L, watchService);
        when(fileWatcherRepository.findByIdOptional(1L)).thenReturn(Optional.empty());

        assertThrows(EntityNotFoundException.class, () -> fileWatcherService.getWatcher(1L));

        watchService.close();
    }

    // --- getWatcherEntities ---

    @Test
    void getWatcherEntities_returnsAllEntities() {
        when(fileWatcherRepository.listAll()).thenReturn(List.of(entityEnabled, entityDisabled));

        Set<FileWatcherEntity> result = fileWatcherService.getWatcherEntities();

        assertEquals(2, result.size());
        assertTrue(result.contains(entityEnabled));
        assertTrue(result.contains(entityDisabled));
    }

    @Test
    void getWatcherEntities_returnsEmptySetWhenNoEntities() {
        when(fileWatcherRepository.listAll()).thenReturn(List.of());

        Set<FileWatcherEntity> result = fileWatcherService.getWatcherEntities();

        assertTrue(result.isEmpty());
    }

    // --- getWatchers ---

    @Test
    void getWatchers_returnsAllFileWatchersWithActiveWatchServices() throws IOException {
        WatchService ws1 = FileSystems.getDefault().newWatchService();
        WatchService ws2 = FileSystems.getDefault().newWatchService();
        fileWatcherService.register(1L, ws1);
        fileWatcherService.register(2L, ws2);
        when(fileWatcherRepository.listAll()).thenReturn(List.of(entityEnabled, entityDisabled));

        Set<FileWatcher> result = fileWatcherService.getWatchers();

        assertEquals(2, result.size());

        ws1.close();
        ws2.close();
    }

    @Test
    void getWatchers_throwsEntityNotFoundExceptionWhenWatchServiceMissing() {
        when(fileWatcherRepository.listAll()).thenReturn(List.of(entityEnabled));
        // No active watcher registered for entityEnabled (id=1)

        assertThrows(EntityNotFoundException.class, () -> fileWatcherService.getWatchers());
    }

    // --- persistFile ---

    @Test
    void persistFile_delegatesToFileService() {
        Path filePath = Path.of("/tmp/source/test.txt");
        when(fileWatcherRepository.findById(1L)).thenReturn(entityEnabled);

        fileWatcherService.persistFile(1L, filePath, false);

        verify(fileService, times(1)).persistFile(entityEnabled, filePath, false);
    }

    @Test
    void persistFile_passesFlushedFlagCorrectly() {
        Path filePath = Path.of("/tmp/source/flushed.txt");
        when(fileWatcherRepository.findById(1L)).thenReturn(entityEnabled);

        fileWatcherService.persistFile(1L, filePath, true);

        verify(fileService, times(1)).persistFile(entityEnabled, filePath, true);
    }
}
