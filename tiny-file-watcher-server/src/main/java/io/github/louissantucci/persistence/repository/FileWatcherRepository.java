package io.github.louissantucci.persistence.repository;

import io.github.louissantucci.persistence.FileWatcherEntity;
import io.quarkus.hibernate.orm.panache.PanacheRepository;
import jakarta.enterprise.context.ApplicationScoped;

@ApplicationScoped
public class FileWatcherRepository implements PanacheRepository<FileWatcherEntity> {}
