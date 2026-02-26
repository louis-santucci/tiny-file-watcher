package io.github.louissantucci.persistence;

import jakarta.persistence.*;
import lombok.*;

import java.util.HashSet;
import java.util.Objects;
import java.util.Set;

@Entity(name = "t_filewatcher_fw")
@Table(name = "t_filewatcher_fw")
@Getter
@Setter
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class FileWatcherEntity {
    @Id
    @GeneratedValue
    private Long id;
    private String source;
    private String destination;
    private boolean enabled;

    @Builder.Default
    @OneToMany(mappedBy = "fileWatcher", cascade = CascadeType.ALL, orphanRemoval = true)
    private Set<FileEntity> files = new HashSet<>();

    @Override
    public boolean equals(Object o) {
        if (o == null || getClass() != o.getClass()) return false;
        FileWatcherEntity entity = (FileWatcherEntity) o;
        return enabled == entity.enabled && Objects.equals(id, entity.id) && Objects.equals(source, entity.source) && Objects.equals(destination, entity.destination);
    }

    @Override
    public int hashCode() {
        return Objects.hash(id, source, destination, enabled);
    }
}
