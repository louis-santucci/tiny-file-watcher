package web

import (
	"context"
	"embed"
	"html/template"
	"net/http"
	"strings"

	pb "tiny-file-watcher/gen/grpc"
)

// templateFS embeds all HTML templates at compile time.
//
//go:embed templates/*.html
var templateFS embed.FS

// Minimal interfaces for the services we need — avoids depending on the
// unexported mustEmbed methods in the generated gRPC server interfaces.
type watcherService interface {
	ListWatchers(context.Context, *pb.ListWatchersRequest) (*pb.ListWatchersResponse, error)
	ToggleWatcher(context.Context, *pb.ToggleWatcherRequest) (*pb.Watcher, error)
}

type flushService interface {
	ListPendingFiles(context.Context, *pb.ListPendingFilesRequest) (*pb.ListPendingFilesResponse, error)
	FlushWatcher(context.Context, *pb.FlushWatcherRequest) (*pb.FlushWatcherResponse, error)
}

type redirectionService interface {
	GetFileRedirection(context.Context, *pb.GetFileRedirectionRequest) (*pb.FileRedirection, error)
}

type filterService interface {
	ListFilters(context.Context, *pb.ListFiltersRequest) (*pb.ListFiltersResponse, error)
}

// Handler holds the HTTP mux and all service dependencies.
type Handler struct {
	mux         *http.ServeMux
	tmpls       map[string]*template.Template
	watcherSvc  watcherService
	flushSvc    flushService
	redirectSvc redirectionService
	filterSvc   filterService
	auth        *authProvider // nil when OIDC is disabled
	sessions    *sessionStore // nil when OIDC is disabled
}

// pages lists the per-page templates that each embed "base.html".
var pages = []string{"index.html", "watchers.html", "watcher.html", "login.html"}

// New wires up the HTTP handler with the given service implementations.
// When oidcCfg.Enabled is true the OIDC provider is initialised and all
// application routes are protected by the requireAuth middleware.
func New(
	watcherSvc watcherService,
	flushSvc flushService,
	redirectSvc redirectionService,
	filterSvc filterService,
	oidcCfg OIDCConfig,
) (*Handler, error) {
	funcs := template.FuncMap{"join": strings.Join}

	tmpls := make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		t, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/base.html", "templates/"+page)
		if err != nil {
			return nil, err
		}
		tmpls[page] = t
	}

	h := &Handler{
		mux:         http.NewServeMux(),
		tmpls:       tmpls,
		watcherSvc:  watcherSvc,
		flushSvc:    flushSvc,
		redirectSvc: redirectSvc,
		filterSvc:   filterSvc,
	}

	if oidcCfg.Enabled {
		ap, err := newAuthProvider(context.Background(), oidcCfg)
		if err != nil {
			return nil, err
		}
		h.auth = ap
		h.sessions = newSessionStore()

		h.mux.HandleFunc("GET /auth/login", h.handleLoginPage)
		h.mux.HandleFunc("GET /auth/authorize", h.handleLogin)
		h.mux.HandleFunc("GET /auth/callback", h.handleCallback)
		h.mux.HandleFunc("POST /auth/logout", h.handleLogout)
	}

	h.mux.HandleFunc("GET /{$}", h.requireAuth(h.handleDashboard))
	h.mux.HandleFunc("GET /watchers", h.requireAuth(h.handleWatcherList))
	h.mux.HandleFunc("GET /watchers/{name}", h.requireAuth(h.handleWatcherDetail))
	h.mux.HandleFunc("POST /watchers/{name}/toggle", h.requireAuth(h.handleToggle))
	h.mux.HandleFunc("POST /watchers/{name}/flush", h.requireAuth(h.handleFlush))

	return h, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	t, ok := h.tmpls[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
