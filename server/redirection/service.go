package redirection

import (
	pb "tiny-file-watcher/gen/grpc"
	"tiny-file-watcher/server/watcher"
)

type RedirectionService struct {
	pb.UnimplementedFileRedirectionServiceServer
	fileWatcherRepository watcher.FileWatcherRepository
	fileRepository        watcher.FileRepository
}
