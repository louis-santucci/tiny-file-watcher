package io.github.louissantucci.persistence.repository;

import io.github.louissantucci.persistence.FileEntity;
import io.quarkus.hibernate.orm.panache.PanacheRepository;
import jakarta.enterprise.context.ApplicationScoped;

@ApplicationScoped
public class FileRepository implements PanacheRepository<FileEntity> {}
