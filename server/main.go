package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"time"
	"tiny-file-watcher/interceptor"

	"github.com/fullstorydev/grpcui/standalone"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"tiny-file-watcher/database"
	pb "tiny-file-watcher/gen/filewatcher"
	"tiny-file-watcher/watcher"
)

const (
	defaultAddr            = ":50051"
	defaultApplicationPath = "/Users/louissantucci/.tfw"
	defaultDBPath          = defaultApplicationPath + "/" + "filewatcher.db"
)

func main() {
	debugUI := flag.Bool("debug-ui", false, "start a grpcui debug web UI on :8080")
	flag.Parse()

	addr := envOrDefault("GRPC_ADDR", defaultAddr)
	dbPath := envOrDefault("DB_PATH", defaultDBPath)

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
		if err := mgr.Start(w.ID, w.SourcePath); err != nil {
			log.Printf("warn: could not resume watcher %s (%s): %v", w.ID, w.SourcePath, err)
		} else {
			log.Printf("resumed watcher %s → %s", w.ID, w.SourcePath)
		}
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v", addr, err)
	}

	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptor.UnaryLoggingInterceptor))
	reflection.Register(grpcServer)
	pb.RegisterFileWatcherServiceServer(grpcServer, NewServer(db, mgr))

	log.Printf("gRPC server listening on %s", addr)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("serve: %v", err)
		}
	}()

	if *debugUI {
		const debugUIAddr = ":8080"
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
	} else {
		select {}
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
