package io.github.louissantucci.persistence;

import jakarta.persistence.*;
import lombok.*;

import java.util.Objects;

@Entity(name = "t_file_fl")
@Table(name = "t_file_fl")
@Getter
@Setter
@NoArgsConstructor
@Builder
@AllArgsConstructor
public class FileEntity {
    @Id
    @GeneratedValue
    private Long id;
    private String path;
    private String filename;
    private boolean isFlushed;

    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "fw_id", nullable = false)
    private FileWatcherEntity fileWatcher;

    @Override
    public boolean equals(Object o) {
        if (o == null || getClass() != o.getClass()) return false;
        FileEntity that = (FileEntity) o;
        return isFlushed == that.isFlushed && Objects.equals(id, that.id) && Objects.equals(fileWatcher, that.fileWatcher) && Objects.equals(path, that.path) && Objects.equals(filename, that.filename);
    }

    @Override
    public int hashCode() {
        return Objects.hash(id, fileWatcher, path, filename, isFlushed);
    }
}
