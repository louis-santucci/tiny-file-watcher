package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	pb "tiny-file-watcher/gen/grpc"

	"github.com/spf13/cobra"
)

var flushCmd = &cobra.Command{
	Use:     "flush",
	Aliases: []string{"f"},
	Short:   "Manage watcher flushes",
}

var flushShowPath bool

func init() {
	flushCmd.AddCommand(
		pendingFilesCmd,
		runFlushCmd,
	)
	pendingFilesCmd.Flags().BoolVarP(&flushShowPath, "show-path", "p", false, "Show the full file path column in the output table")
}

var pendingFilesCmd = &cobra.Command{
	Use:   "pending <watcher-name>",
	Short: "List pending (unflushed) files for a watcher",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewFileFlushServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := svc.ListPendingFiles(ctx, &pb.ListPendingFilesRequest{Name: args[0]})
		if err != nil {
			return err
		}

		if len(resp.Files) == 0 {
			fmt.Printf("No pending files for watcher %q.\n", args[0])
			return nil
		}

		printFlushedWatchedFiles(resp.Files, flushShowPath)
		return nil
	},
}

var runFlushCmd = &cobra.Command{
	Use:   "run <watcher-name>",
	Short: "Flush pending files for a watcher",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewFileFlushServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := svc.FlushWatcher(ctx, &pb.FlushWatcherRequest{Name: args[0]})
		if err != nil {
			return err
		}

		if resp.Success {
			fmt.Printf("Watcher %q flushed successfully.\n", args[0])
		} else {
			fmt.Printf("Watcher %q could not be flushed.\n", args[0])
		}
		return nil
	},
}

func printFlushedWatchedFiles(files []*pb.WatchedFile, showPath bool) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if showPath {
		fmt.Fprintln(w, "ID\tWATCHER\tFILE NAME\tFILE PATH")
		fmt.Fprintln(w, "--\t-------\t---------\t---------")
		for _, f := range files {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", f.Id, f.WatcherId, f.FileName, shortenPath(f.FilePath, 40))
		}
	} else {
		fmt.Fprintln(w, "ID\tWATCHER\tFILE NAME")
		fmt.Fprintln(w, "--\t-------\t---------")
		for _, f := range files {
			fmt.Fprintf(w, "%d\t%s\t%s\n", f.Id, f.WatcherId, f.FileName)
		}
	}
	w.Flush()
}

func shortenPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}
