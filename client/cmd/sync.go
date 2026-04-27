package cmd

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	pb "tiny-file-watcher/gen/grpc"
)

var syncNoStream bool

var syncWatcherCmd = &cobra.Command{
	Use:   "sync <name>",
	Short: "Sync a watcher by scanning its source directory",
	Long: `Sync a watcher by scanning its source directory.

By default the command uses the streaming RPC (StreamSyncWatcher) and prints
progress messages as they arrive.  Pass --no-stream to fall back to the
unary RPC (SyncWatcher) for a single-response call.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewFileWatcherServiceClient(conn)

		if syncNoStream {
			return runUnarySyncWatcher(svc, args[0])
		}
		return runStreamSyncWatcher(svc, args[0])
	},
}

func init() {
	syncWatcherCmd.Flags().BoolVar(&syncNoStream, "no-stream", false, "Use the unary SyncWatcher RPC instead of the default streaming RPC")
}

// runUnarySyncWatcher calls the unary SyncWatcher RPC and prints the result.
func runUnarySyncWatcher(svc pb.FileWatcherServiceClient, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := svc.SyncWatcher(ctx, &pb.SyncWatcherRequest{
		Name: name,
	})
	if err != nil {
		return err
	}

	printSyncResult(name, resp.AddedCount, resp.RemovedCount, resp.AddedFiles, resp.RemovedFiles)
	return nil
}

// runStreamSyncWatcher calls the server-streaming StreamSyncWatcher RPC,
// prints LOG events as they arrive, and prints the final summary from the
// RESULT event.
func runStreamSyncWatcher(svc pb.FileWatcherServiceClient, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	stream, err := svc.StreamSyncWatcher(ctx, &pb.SyncWatcherRequest{
		Name: name,
	})
	if err != nil {
		return err
	}

	for {
		event, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch event.Type {
		case pb.SyncWatcherEvent_LOG:
			fmt.Printf("[sync] %s\n", event.Message)
		case pb.SyncWatcherEvent_RESULT:
			if r := event.Result; r != nil {
				printSyncResult(name, r.AddedCount, r.RemovedCount, r.AddedFiles, r.RemovedFiles)
			}
		}
	}

	return nil
}

// printSyncResult prints a sync summary in the same format for both unary and streaming paths.
func printSyncResult(name string, addedCount, removedCount int64, addedFiles, removedFiles []string) {
	fmt.Printf("Sync complete for watcher %q:\n", name)
	fmt.Printf("  Added:   %d file(s)\n", addedCount)
	fmt.Printf("  Removed: %d file(s)\n", removedCount)
	if len(addedFiles) > 0 {
		fmt.Println("  Added files:")
		for _, f := range addedFiles {
			fmt.Printf("    + %s\n", shortenPath(f, 40))
		}
	}
	if len(removedFiles) > 0 {
		fmt.Println("  Removed files:")
		for _, f := range removedFiles {
			fmt.Printf("    - %s\n", shortenPath(f, 40))
		}
	}
}
