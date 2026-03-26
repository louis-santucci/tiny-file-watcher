package cmd

import (
	"context"
	"fmt"
	"time"
	pb "tiny-file-watcher/gen/grpc"

	"github.com/spf13/cobra"
)

var syncWatcherCmd = &cobra.Command{
	Use:   "sync <name>",
	Short: "Sync a watcher by scanning its source directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewFileWatcherServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resp, err := svc.SyncWatcher(ctx, &pb.SyncWatcherRequest{Name: args[0]})
		if err != nil {
			return err
		}

		fmt.Printf("Sync complete for watcher %q:\n", args[0])
		fmt.Printf("  Added:   %d file(s)\n", resp.AddedCount)
		fmt.Printf("  Removed: %d file(s)\n", resp.RemovedCount)
		if len(resp.AddedFiles) > 0 {
			fmt.Println("  Added files:")
			for _, f := range resp.AddedFiles {
				fmt.Printf("    + %s\n", shortenPath(f, 40))
			}
		}
		if len(resp.RemovedFiles) > 0 {
			fmt.Println("  Removed files:")
			for _, f := range resp.RemovedFiles {
				fmt.Printf("    - %s\n", shortenPath(f, 40))
			}
		}
		return nil
	},
}
