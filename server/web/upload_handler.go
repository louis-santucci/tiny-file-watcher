package web

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	pb "tiny-file-watcher/gen/grpc"
)

const maxUploadSize = 64 << 20 // 64 MB

// resolveUploadDir returns the absolute destination directory for an upload.
// folder must be a plain directory name (no slashes, no path traversal).
// When folder is empty the watcher's source_path is returned as-is.
func resolveUploadDir(sourcePath, folder string) (string, error) {
	if folder == "" {
		return sourcePath, nil
	}
	if strings.ContainsAny(folder, `/\`) || folder == ".." {
		return "", fmt.Errorf("invalid folder: must be a single directory name")
	}
	resolved := filepath.Join(sourcePath, folder)
	if !strings.HasPrefix(resolved, sourcePath) {
		return "", fmt.Errorf("invalid folder: path traversal detected")
	}
	return resolved, nil
}

// handleUpload saves uploaded files directly into the watcher's source_path (or
// an optional subfolder). After uploading, call SyncWatcher to record the new
// files in the database.
func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	resp, err := h.watcherSvc.ListWatchers(r.Context(), &pb.ListWatchersRequest{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var wt *pb.Watcher
	for _, candidate := range resp.Watchers {
		if candidate.Name == name {
			wt = candidate
			break
		}
	}
	if wt == nil {
		http.Error(w, "watcher not found", http.StatusNotFound)
		return
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `<span style="color:var(--pico-color-red-550)">Upload failed: %s</span>`, err.Error())
		return
	}

	folder := r.FormValue("folder")
	destDir, err := resolveUploadDir(wt.SourcePath, folder)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `<span style="color:var(--pico-color-red-550)">Invalid folder: %s</span>`, err.Error())
		return
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `<span style="color:var(--pico-color-red-550)">No files selected.</span>`)
		return
	}

	var saved []string
	for _, fh := range files {
		src, err := fh.Open()
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `<span style="color:var(--pico-color-red-550)">Could not read %s: %s</span>`, fh.Filename, err.Error())
			return
		}
		defer src.Close()

		destPath := filepath.Join(destDir, filepath.Base(fh.Filename))
		dst, err := os.Create(destPath)
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `<span style="color:var(--pico-color-red-550)">Could not save %s: %s</span>`, fh.Filename, err.Error())
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `<span style="color:var(--pico-color-red-550)">Could not write %s: %s</span>`, fh.Filename, err.Error())
			return
		}

		saved = append(saved, fh.Filename)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w,
		`<span style="color:var(--pico-color-green-550)">✓ Uploaded: %s</span>`,
		strings.Join(saved, ", "),
	)
}
