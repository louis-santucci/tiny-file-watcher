package io.github.louissantucci.grpc;

import io.github.louissantucci.service.FileWatcherService;
import io.quarkus.grpc.GrpcService;
import io.smallrye.mutiny.Uni;
import jakarta.persistence.EntityNotFoundException;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;

import java.util.List;
import java.util.Objects;

@GrpcService
@Slf4j
@RequiredArgsConstructor
public class FileWatcherGrpcService implements FileWatcherCrud {

    private final FileWatcherService fileWatcherService;

    @Override
    public Uni<FileWatcherListResponse> getAllFileWatchers(GetAllFileWatchersRequest request) {
        return Uni.createFrom().item(() -> {
            try {
                List<FileWatcherMessage> messages = fileWatcherService.getWatcherEntities().stream()
                        .map(entity -> FileWatcherMessage.newBuilder()
                                .setId(entity.getId())
                                .setSource(entity.getSource())
                                .setDestination(entity.getDestination())
                                .setEnabled(fileWatcherService.isEnabled(entity.getId()))
                                .build())
                        .filter(msg -> !request.hasActive() || Objects.equals(msg.getEnabled(), request.getActive()))
                        .toList();

                return FileWatcherListResponse.newBuilder()
                        .setStatus(Status.SUCCESS)
                        .addAllFileWatchers(messages)
                        .build();
            } catch (Exception e) {
                log.error("GetAllFileWatchers failed: {}", e.getMessage(), e);
                return FileWatcherListResponse.newBuilder()
                        .setStatus(Status.ERROR)
                        .setErrorMessage(e.getMessage())
                        .build();
            }
        });
    }

    @Override
    public Uni<FileWatcherResponse> getFileWatcher(GetFileWatcherRequest request) {
        return Uni.createFrom().item(() -> {
            try {
                var entity = fileWatcherService.getWatcherEntities().stream()
                        .filter(e -> e.getId() == request.getId())
                        .findFirst()
                        .orElseThrow(() -> new EntityNotFoundException(
                                "File watcher with id " + request.getId() + " not found"));

                FileWatcherMessage message = FileWatcherMessage.newBuilder()
                        .setId(entity.getId())
                        .setSource(entity.getSource())
                        .setDestination(entity.getDestination())
                        .setEnabled(fileWatcherService.isEnabled(entity.getId()))
                        .build();

                return FileWatcherResponse.newBuilder()
                        .setStatus(Status.SUCCESS)
                        .setFileWatcher(message)
                        .build();
            } catch (EntityNotFoundException e) {
                log.warn("GetFileWatcher [id={}] not found: {}", request.getId(), e.getMessage());
                return FileWatcherResponse.newBuilder()
                        .setStatus(Status.ERROR)
                        .setErrorMessage(e.getMessage())
                        .build();
            } catch (Exception e) {
                log.error("GetFileWatcher [id={}] failed: {}", request.getId(), e.getMessage(), e);
                return FileWatcherResponse.newBuilder()
                        .setStatus(Status.ERROR)
                        .setErrorMessage(e.getMessage())
                        .build();
            }
        });
    }

    @Override
    public Uni<FileWatcherResponse> createFileWatcher(CreateFileWatcherRequest request) {
        return Uni.createFrom().item(() -> {
            try {
                long id = fileWatcherService.createWatcher(request.getSource(), request.getDestination());

                FileWatcherMessage message = FileWatcherMessage.newBuilder()
                        .setId(id)
                        .setSource(request.getSource())
                        .setDestination(request.getDestination())
                        .setEnabled(false)
                        .build();

                return FileWatcherResponse.newBuilder()
                        .setStatus(Status.SUCCESS)
                        .setFileWatcher(message)
                        .build();
            } catch (Exception e) {
                log.error("CreateFileWatcher failed: {}", e.getMessage(), e);
                return FileWatcherResponse.newBuilder()
                        .setStatus(Status.ERROR)
                        .setErrorMessage(e.getMessage())
                        .build();
            }
        });
    }

    @Override
    public Uni<DeleteFileWatcherResponse> deleteFileWatcher(DeleteFileWatcherRequest request) {
        return Uni.createFrom().item(() -> {
            try {
                fileWatcherService.deleteWatcher(request.getId());
                return DeleteFileWatcherResponse.newBuilder()
                        .setStatus(Status.SUCCESS)
                        .build();
            } catch (EntityNotFoundException e) {
                log.warn("DeleteFileWatcher [id={}] not found: {}", request.getId(), e.getMessage());
                return DeleteFileWatcherResponse.newBuilder()
                        .setStatus(Status.ERROR)
                        .setErrorMessage(e.getMessage())
                        .build();
            } catch (Exception e) {
                log.error("DeleteFileWatcher [id={}] failed: {}", request.getId(), e.getMessage(), e);
                return DeleteFileWatcherResponse.newBuilder()
                        .setStatus(Status.ERROR)
                        .setErrorMessage(e.getMessage())
                        .build();
            }
        });
    }
}
