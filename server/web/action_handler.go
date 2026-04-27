package web

import (
	"fmt"
	"net/http"

	pb "tiny-file-watcher/gen/grpc"
)

// handleSync triggers a directory sync and returns an HTMX partial (watcher row).
func (h *Handler) handleSync(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	result, err := h.watcherSvc.SyncWatcher(r.Context(), &pb.SyncWatcherRequest{Name: name})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, syncResultPartial(name, result))
}

// handleFlush triggers a flush and returns an HTMX partial (pending files list).
func (h *Handler) handleFlush(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	_, err := h.flushSvc.FlushWatcher(r.Context(), &pb.FlushWatcherRequest{Name: name})
	if err != nil && !isNotFound(err) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pf, err := h.flushSvc.ListPendingFiles(r.Context(), &pb.ListPendingFilesRequest{Name: name})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, pendingFilesPartial(pf.Files))
}

func syncResultPartial(name string, result *pb.SyncWatcherResponse) string {
	return fmt.Sprintf(`<div id="sync-result-%s">
  <p>Sync complete: <strong>+%d</strong> added, <strong>-%d</strong> removed.</p>
  <button hx-post="/watchers/%s/sync"
          hx-target="#sync-result-%s"
          hx-swap="outerHTML"
          class="secondary small">Sync</button>
</div>`, name, result.AddedCount, result.RemovedCount, name, name)
}

func pendingFilesPartial(files []*pb.WatchedFile) string {
	if len(files) == 0 {
		return `<p id="pending-files"><em>No pending files.</em></p>`
	}
	out := `<ul id="pending-files">`
	for _, f := range files {
		out += fmt.Sprintf("<li>%s</li>", f.FileName)
	}
	out += "</ul>"
	return out
}
