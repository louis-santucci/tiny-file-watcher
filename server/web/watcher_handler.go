package web

import (
	"net/http"

	pb "tiny-file-watcher/gen/grpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type dashboardData struct {
	Total   int
	Enabled int
	Pending int
}

func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	resp, err := h.watcherSvc.ListWatchers(r.Context(), &pb.ListWatchersRequest{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := dashboardData{Total: len(resp.Watchers)}
	for _, wt := range resp.Watchers {
		if wt.Enabled {
			data.Enabled++
		}
		pf, err := h.flushSvc.ListPendingFiles(r.Context(), &pb.ListPendingFilesRequest{Name: wt.Name})
		if err == nil {
			data.Pending += len(pf.Files)
		}
	}

	h.render(w, "index.html", data)
}

type watcherListData struct {
	Watchers []*pb.Watcher
}

func (h *Handler) handleWatcherList(w http.ResponseWriter, r *http.Request) {
	resp, err := h.watcherSvc.ListWatchers(r.Context(), &pb.ListWatchersRequest{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "watchers.html", watcherListData{Watchers: resp.Watchers})
}

type watcherDetailData struct {
	Watcher     *pb.Watcher
	Redirection *pb.FileRedirection
	Filters     []*pb.WatcherFilter
	Pending     []*pb.WatchedFile
}

func (h *Handler) handleWatcherDetail(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	resp, err := h.watcherSvc.ListWatchers(r.Context(), &pb.ListWatchersRequest{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var wt *pb.Watcher
	for _, w := range resp.Watchers {
		if w.Name == name {
			wt = w
			break
		}
	}
	if wt == nil {
		http.Error(w, "watcher not found", http.StatusNotFound)
		return
	}

	data := watcherDetailData{Watcher: wt}

	if red, err := h.redirectSvc.GetFileRedirection(r.Context(), &pb.GetFileRedirectionRequest{Name: name}); err == nil {
		data.Redirection = red
	}

	if filters, err := h.filterSvc.ListFilters(r.Context(), &pb.ListFiltersRequest{WatcherName: name}); err == nil {
		data.Filters = filters.Filters
	}

	if pf, err := h.flushSvc.ListPendingFiles(r.Context(), &pb.ListPendingFilesRequest{Name: name}); err == nil {
		data.Pending = pf.Files
	}

	h.render(w, "watcher.html", data)
}

// isNotFound returns true when a gRPC status is codes.NotFound.
func isNotFound(err error) bool {
	if s, ok := status.FromError(err); ok {
		return s.Code() == codes.NotFound
	}
	return false
}
