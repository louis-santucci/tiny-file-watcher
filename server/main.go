package main

import (
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
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

	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)
	pb.RegisterFileWatcherServiceServer(grpcServer, NewServer(db, mgr))

	log.Printf("gRPC server listening on %s", addr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
