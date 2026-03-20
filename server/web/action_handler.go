package web

import (
	"fmt"
	"net/http"

	pb "tiny-file-watcher/gen/grpc"
)

// handleToggle toggles a watcher on/off and returns an HTMX partial (status badge).
func (h *Handler) handleToggle(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	wt, err := h.watcherSvc.ToggleWatcher(r.Context(), &pb.ToggleWatcherRequest{Name: name})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, watcherRowPartial(wt))
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

func watcherRowPartial(wt *pb.Watcher) string {
	badge, toggleLabel := statusBadge(wt.Enabled)
	return fmt.Sprintf(`<tr id="watcher-row-%s">
  <td><a href="/watchers/%s">%s</a></td>
  <td>%s</td>
  <td>%s</td>
  <td>
    <button hx-post="/watchers/%s/toggle"
            hx-target="#watcher-row-%s"
            hx-swap="outerHTML"
            class="secondary small">%s</button>
  </td>
</tr>`, wt.Name, wt.Name, wt.Name, wt.SourcePath, badge, wt.Name, wt.Name, toggleLabel)
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

func statusBadge(enabled bool) (badge, toggleLabel string) {
	if enabled {
		return `<span style="color:var(--pico-color-green-550)">● Enabled</span>`, "Disable"
	}
	return `<span style="color:var(--pico-color-red-550)">● Disabled</span>`, "Enable"
}
