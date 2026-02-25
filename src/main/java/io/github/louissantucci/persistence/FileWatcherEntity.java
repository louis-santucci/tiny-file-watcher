package io.github.louissantucci.persistence;

import jakarta.persistence.*;
import lombok.Getter;
import lombok.Setter;

import java.util.HashSet;
import java.util.Objects;
import java.util.Set;

@Entity(name = "t_filewatcher_fw")
@Table(name = "t_filewatcher_fw")
@Getter
@Setter
public class FileWatcherEntity {
    @Id
    @GeneratedValue
    private Long id;
    private String source;
    private String destination;

    @OneToMany(mappedBy = "fileWatcher", cascade = CascadeType.ALL, orphanRemoval = true)
    private Set<FileEntity> files = new HashSet<>();

    @Override
    public boolean equals(Object o) {
        if (o == null || getClass() != o.getClass()) return false;
        FileWatcherEntity that = (FileWatcherEntity) o;
        return Objects.equals(id, that.id) && Objects.equals(source, that.source) && Objects.equals(destination, that.destination);
    }

    @Override
    public int hashCode() {
        return Objects.hash(id, source, destination);
    }
}
