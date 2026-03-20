package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"time"
	pb "tiny-file-watcher/gen/grpc"
	config2 "tiny-file-watcher/server/config"
	"tiny-file-watcher/server/database"
	"tiny-file-watcher/server/filter"
	"tiny-file-watcher/server/flush"
	"tiny-file-watcher/server/interceptor"
	"tiny-file-watcher/server/redirection"
	"tiny-file-watcher/server/watcher"
	"tiny-file-watcher/server/web"

	"github.com/fullstorydev/grpcui/standalone"
	"github.com/ridgelines/go-config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

// App holds all application-level components.
type App struct {
	config     *config.Config
	db         *database.DB
	mgr        *watcher.Manager
	grpcServer *grpc.Server
	grpcAddr   string
	webHandler *web.Handler
}

// NewApp loads configuration, opens the database, wires up all components,
// and returns a fully initialised App ready to Run.
func NewApp() (*App, error) {
	cfg := config2.InitConfig()

	if err := cfg.Load(); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	logLevel := slog.LevelInfo
	if levelStr, err := cfg.String("log.level"); err == nil {
		if err := logLevel.UnmarshalText([]byte(levelStr)); err != nil {
			return nil, fmt.Errorf("invalid log.level %q: %w", levelStr, err)
		}

	}
	config2.InitLogging(logLevel)
	logger := slog.Default()
	log.Printf("log_level=%s", logLevel)

	grpcAddr, _ := cfg.String("grpc.address")

	dbName, _ := cfg.String("db.name")
	dbPath := config2.DefaultDBPath + "/" + dbName

	db, err := database.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	mgr := watcher.NewManager(db, db, logger)

	// Resume any watchers that were enabled before the last shutdown.
	enabled, err := db.ListEnabledWatchers()
	if err != nil {
		return nil, fmt.Errorf("list enabled watchers: %w", err)
	}
	for _, w := range enabled {
		key := watcher.WatcherKey{Id: w.ID, Name: w.Name}
		if err := mgr.Start(key, w.SourcePath); err != nil {
			log.Printf("warn: could not resume watcher %s (id %d) (%s): %v", w.Name, w.ID, w.SourcePath, err)
		} else {
			log.Printf("resumed watcher %s (id %d) → %s", w.Name, w.ID, w.SourcePath)
		}
	}

	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptor.UnaryLoggingInterceptor))
	reflection.Register(grpcServer)
	watcherSvc := watcher.NewManagerService(db, db, mgr, logger)
	redirectionSvc := redirection.NewRedirectionService(db, db, db, logger)
	flushSvc := flush.NewFlushService(db, logger)
	filterSvc := filter.NewFilterService(db, logger)
	pb.RegisterFileWatcherServiceServer(grpcServer, watcherSvc)
	pb.RegisterFileRedirectionServiceServer(grpcServer, redirectionSvc)
	pb.RegisterFileFlushServiceServer(grpcServer, flushSvc)
	pb.RegisterWatcherFilterServiceServer(grpcServer, filterSvc)

	webHandler, err := web.New(watcherSvc, flushSvc, redirectionSvc, filterSvc)
	if err != nil {
		return nil, fmt.Errorf("create web handler: %w", err)
	}

	return &App{
		config:     cfg,
		db:         db,
		mgr:        mgr,
		grpcServer: grpcServer,
		grpcAddr:   grpcAddr,
		webHandler: webHandler,
	}, nil
}

// Run starts the gRPC server and, if enabled, the debug UI and web UI. It
// blocks until all servers exit (or indefinitely when both UIs are disabled).
func (a *App) Run() error {
	lis, err := net.Listen("tcp", a.grpcAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", a.grpcAddr, err)
	}

	log.Printf("gRPC server listening on %s", a.grpcAddr)
	go func() {
		if err := a.grpcServer.Serve(lis); err != nil {
			log.Fatalf("serve: %v", err)
		}
	}()

	webEnabled, _ := a.config.Bool("web.enabled")
	if webEnabled {
		webAddr, _ := a.config.String("web.address")
		log.Printf("web UI available at http://localhost%s", webAddr)
		go func() {
			if err := http.ListenAndServe(webAddr, a.webHandler); err != nil {
				log.Fatalf("web UI: serve: %v", err)
			}
		}()
	}

	debugUI, _ := a.config.Bool("debug-ui.enabled")
	if debugUI {
		return a.enableDebugUI()
	}

	// Block forever when neither UI is serving as foreground.
	select {}
}

// enableDebugUI dials the gRPC server via reflection and serves the web UI.
func (a *App) enableDebugUI() error {
	debugUIAddr, _ := a.config.String("debug-ui.address")

	// Give the gRPC server a moment to be ready before dialing.
	time.Sleep(100 * time.Millisecond)

	cc, err := grpc.NewClient(a.grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("debug-ui: dial gRPC: %w", err)
	}
	defer func() {
		if err := cc.Close(); err != nil {
			log.Printf("debug-ui: close gRPC conn: %v", err)
		}
	}()

	h, err := standalone.HandlerViaReflection(context.Background(), cc, a.grpcAddr)
	if err != nil {
		return fmt.Errorf("debug-ui: create handler: %w", err)
	}

	log.Printf("gRPC debug UI available at http://localhost%s", debugUIAddr)
	if err := http.ListenAndServe(debugUIAddr, h); err != nil {
		return fmt.Errorf("debug-ui: serve: %w", err)
	}
	return nil
}
