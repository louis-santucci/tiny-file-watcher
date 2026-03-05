package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
	pb "tiny-file-watcher/gen/grpc"
	config2 "tiny-file-watcher/server/config"
	"tiny-file-watcher/server/database"
	"tiny-file-watcher/server/interceptor"
	"tiny-file-watcher/server/watcher"

	"github.com/fullstorydev/grpcui/standalone"
	"github.com/ridgelines/go-config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

//go:embed banner.txt
var banner string

func main() {
	fmt.Println(banner)
	config := config2.InitConfig()
	config2.InitLogging()
	err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	addr, _ := config.String("grpc.address")

	dbName, _ := config.String("db.name")
	dbPath := config2.DefaultDBPath + "/" + dbName

	db, err := database.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	mgr := watcher.NewManager(db)

	// Resume any watchers that were enabled before the last shutdown.
	enabled, err := db.ListEnabledWatchers()
	if err != nil {
		log.Fatalf("list enabled watchers: %v", err)
	}
	for _, w := range enabled {
		key := watcher.WatcherKey{Id: w.ID, Name: w.Name}
		if err := mgr.Start(key, w.SourcePath); err != nil {
			log.Printf("warn: could not resume watcher %s (id %d) (%s): %v", w.Name, w.ID, w.SourcePath, err)
		} else {
			log.Printf("resumed watcher %s (id %d) → %s", w.Name, w.ID, w.SourcePath)
		}
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v", addr, err)
	}

	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptor.UnaryLoggingInterceptor))
	reflection.Register(grpcServer)
	pb.RegisterFileWatcherServiceServer(grpcServer, watcher.NewManagerService(db, db, mgr))

	log.Printf("gRPC server listening on %s", addr)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("serve: %v", err)
		}
	}()

	debugUI, _ := config.Bool("debug-ui.enabled")

	if debugUI {
		enableDebugUI(addr, config)
	}
}

func enableDebugUI(addr string, config *config.Config) {
	debugUIAddr, _ := config.String("debug-ui.address")

	// Give the gRPC server a moment to be ready before dialing.
	time.Sleep(100 * time.Millisecond)
	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("debug-ui: dial gRPC: %v", err)
	}
	defer cc.Close()
	h, err := standalone.HandlerViaReflection(context.Background(), cc, addr)
	if err != nil {
		log.Fatalf("debug-ui: create handler: %v", err)
	}
	log.Printf("gRPC debug UI available at http://localhost%s", debugUIAddr)
	if err := http.ListenAndServe(debugUIAddr, h); err != nil {
		log.Fatalf("debug-ui: serve: %v", err)
	}
}
