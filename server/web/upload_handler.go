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

// handleUpload saves uploaded files directly into the watcher's source_path.
// The watcher's fsnotify goroutine will detect the new files and record them.
// Upload is rejected if the watcher is not enabled.
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
	if !wt.Enabled {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `<span style="color:var(--pico-color-red-550)">Upload failed: watcher is disabled. Enable the watcher first.</span>`)
		return
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `<span style="color:var(--pico-color-red-550)">Upload failed: %s</span>`, err.Error())
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

		destPath := filepath.Join(wt.SourcePath, filepath.Base(fh.Filename))
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
