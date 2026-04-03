package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"time"
	"tiny-file-watcher/internal"

	pb "tiny-file-watcher/gen/grpc"
	config2 "tiny-file-watcher/server/config"
	"tiny-file-watcher/server/database"
	"tiny-file-watcher/server/flush"
	"tiny-file-watcher/server/interceptor"
	"tiny-file-watcher/server/machine"
	"tiny-file-watcher/server/redirection"
	"tiny-file-watcher/server/watcher"
	"tiny-file-watcher/server/web"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/fullstorydev/grpcui/standalone"
	"github.com/ridgelines/go-config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

var validator = config2.ServerConfigValidator

// App holds all application-level components.
type App struct {
	config     *config.Config
	db         *database.DB
	grpcServer *grpc.Server
	grpcAddr   string
	webHandler *web.Handler
}

// NewApp loads configuration, opens the database, wires up all components,
// and returns a fully initialised App ready to Run.
func NewApp() (*App, error) {
	cfg := internal.InitConfig(&validator)

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
	slog.Debug("gRPC address: " + grpcAddr)

	dbPath, _ := cfg.StringOr("database.path", internal.DatabasePath())
	slog.Debug("database path: " + dbPath)
	db, err := database.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	oidcCfg := oidcCfgFromConfig(cfg)

	var tokenVerifier interceptor.TokenVerifier
	if oidcCfg.Enabled {
		provider, err := gooidc.NewProvider(context.Background(), oidcCfg.Issuer)
		if err != nil {
			return nil, fmt.Errorf("oidc provider discovery: %w", err)
		}
		v := provider.Verifier(&gooidc.Config{ClientID: oidcCfg.DeviceClientID})
		tokenVerifier = interceptor.NewOIDCTokenVerifier(v)
	} else {
		tokenVerifier = interceptor.NewNoopVerifier()
	}

	unaryAuth, streamAuthInterceptor := interceptor.NewAuthInterceptors(tokenVerifier)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(interceptor.UnaryLoggingInterceptor, unaryAuth),
		grpc.ChainStreamInterceptor(streamAuthInterceptor),
	)
	reflection.Register(grpcServer)
	watcherSvc := watcher.NewManagerService(db, db, db, logger)
	redirectionSvc := redirection.NewRedirectionService(db, db, db, logger)
	flushSvc := flush.NewFlushService(db, logger)
	machineSvc := machine.NewMachineService(db, logger)
	pb.RegisterFileWatcherServiceServer(grpcServer, watcherSvc)
	pb.RegisterFileRedirectionServiceServer(grpcServer, redirectionSvc)
	pb.RegisterFileFlushServiceServer(grpcServer, flushSvc)
	pb.RegisterMachineServiceServer(grpcServer, machineSvc)

	webHandler, err := web.New(watcherSvc, flushSvc, redirectionSvc, oidcCfg)
	if err != nil {
		return nil, fmt.Errorf("create web handler: %w", err)
	}

	return &App{
		config:     cfg,
		db:         db,
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

// oidcCfgFromConfig reads OIDC settings from the application config.
func oidcCfgFromConfig(cfg *config.Config) web.OIDCConfig {
	enabled, _ := cfg.Bool("oidc.enabled")
	issuer, _ := cfg.String("oidc.issuer")
	clientID, _ := cfg.String("oidc.client-id")
	deviceClientID, _ := cfg.String("oidc.device-client-id")
	clientSecret, _ := cfg.String("oidc.client-secret")
	redirectURI, _ := cfg.String("oidc.redirect-uri")
	return web.OIDCConfig{
		Enabled:        enabled,
		Issuer:         issuer,
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		DeviceClientID: deviceClientID,
		RedirectURI:    redirectURI,
	}
}
