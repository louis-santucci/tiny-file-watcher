package cmd

import (
	"context"
	"fmt"
	"io"
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

var (
	flushPendingShowPath bool
	flushNoStream        bool
)

func init() {
	pendingFilesCmd.Flags().BoolVarP(&flushPendingShowPath, "show-path", "p", false, "Show the full file path column in the output table")
	runFlushCmd.Flags().BoolVar(&flushNoStream, "no-stream", false, "Use the unary RPC instead of the streaming RPC")

	flushCmd.AddCommand(
		pendingFilesCmd,
		runFlushCmd,
	)
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

		printFlushedWatchedFiles(resp.Files, flushPendingShowPath)
		return nil
	},
}

var runFlushCmd = &cobra.Command{
	Use:   "run <watcher-name>",
	Short: "Flush pending files for a watcher",
	Long: `Flush pending files for a watcher.

By default the command uses the streaming RPC (StreamFlushWatcher) and prints
progress messages as they arrive.  Pass --no-stream to fall back to the
unary RPC (FlushWatcher) for a single-response call.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewFileFlushServiceClient(conn)

		if flushNoStream {
			return runUnaryFlushWatcher(svc, args[0])
		}
		return runStreamFlushWatcher(svc, args[0])
	},
}

// runUnaryFlushWatcher calls the unary FlushWatcher RPC and prints the result.
func runUnaryFlushWatcher(svc pb.FileFlushServiceClient, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	resp, err := svc.FlushWatcher(ctx, &pb.FlushWatcherRequest{Name: name})
	if err != nil {
		return err
	}

	printFlushResult(name, resp.Success)
	return nil
}

// runStreamFlushWatcher calls the server-streaming StreamFlushWatcher RPC,
// prints LOG events as they arrive, and prints the final result from the
// RESULT event.
func runStreamFlushWatcher(svc pb.FileFlushServiceClient, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	stream, err := svc.StreamFlushWatcher(ctx, &pb.FlushWatcherRequest{Name: name})
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
		case pb.FlushWatcherEvent_LOG:
			fmt.Printf("[flush] %s\n", event.Message)
		case pb.FlushWatcherEvent_RESULT:
			if r := event.Result; r != nil {
				printFlushResult(name, r.Success)
			}
		}
	}

	return nil
}

// printFlushResult prints the flush outcome in the same format for both unary and streaming paths.
func printFlushResult(name string, success bool) {
	if success {
		fmt.Printf("Watcher %q flushed successfully.\n", name)
	} else {
		fmt.Printf("Watcher %q could not be flushed.\n", name)
	}
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
