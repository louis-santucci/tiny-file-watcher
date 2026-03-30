package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	clientmachine "tiny-file-watcher/client/machine"
	pb "tiny-file-watcher/gen/grpc"

	"github.com/spf13/cobra"
)

var watcherCmd = &cobra.Command{
	Use:     "watcher",
	Aliases: []string{"w"},
	Short:   "Manage file watchers",
}

var watchedFilsShowPath bool
var listWatchersAllMachines bool

func init() {
	// create
	createWatcherCmd.Flags().StringP("path", "p", "", "Source path to watch (required)")
	_ = createWatcherCmd.MarkFlagRequired("path")
	createWatcherCmd.Flags().Bool("flush-existing", false, "Add files already on disk as pending (to be flushed); by default they are recorded as already flushed")
	createWatcherCmd.Flags().StringArrayP("filter", "f", nil,
		`Filter rule in the form rule_type:pattern_type:pattern (repeatable).
    rule_type    : include | exclude
    pattern_type : extension | name | glob
    Example: --filter include:extension:.go --filter exclude:glob:*_test.go`)

	// update
	updateWatcherCmd.Flags().String("name", "", "New name for the watcher")
	updateWatcherCmd.Flags().String("path", "", "New source path for the watcher")

	listWatcherFilesCmd.Flags().BoolVarP(&watchedFilsShowPath, "show-path", "p", false, "Show the full file path column in the output table")
	listWatchersCmd.Flags().BoolVarP(&listWatchersAllMachines, "all", "a", false, "List watchers from all machines (default: current machine only)")

	watcherCmd.AddCommand(
		listWatcherFilesCmd,
		listWatchersCmd,
		getWatcherCmd,
		createWatcherCmd,
		updateWatcherCmd,
		deleteWatcherCmd,
		syncWatcherCmd,
	)
}

var listWatcherFilesCmd = &cobra.Command{
	Use:   "files <watcher-name>",
	Short: "List all files tracked by a watcher",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewFileWatcherServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := svc.ListWatchedFiles(ctx, &pb.ListWatchedFilesRequest{WatcherName: args[0]})
		if err != nil {
			return err
		}

		printWatchedFiles(resp.Files, watchedFilsShowPath)
		return nil
	},
}

var listWatchersCmd = &cobra.Command{
	Use:   "list",
	Short: "List all watchers",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewFileWatcherServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		req := &pb.ListWatchersRequest{}
		if !listWatchersAllMachines {
			machineName, err := clientmachine.LoadMachineName()
			if err == nil {
				req.MachineName = &machineName
			} else {
				fmt.Fprintln(os.Stderr, "note: machine not initialized, listing watchers from all machines (run 'tfw machine init' to associate a machine)")
			}
		}

		resp, err := svc.ListWatchers(ctx, req)
		if err != nil {
			return err
		}

		printWatchers(resp.Watchers)
		return nil
	},
}

var getWatcherCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Get a watcher by name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		watcherSvc := pb.NewFileWatcherServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		w, err := watcherSvc.GetWatcherByName(ctx, &pb.GetWatcherByNameRequest{Name: args[0]})
		if err != nil {
			return err
		}

		printWatchers([]*pb.Watcher{w})
		return nil
	},
}

var createWatcherCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new watcher",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("path")
		flushExisting, _ := cmd.Flags().GetBool("flush-existing")
		filters, _ := cmd.Flags().GetStringArray("filter")

		machineName, err := clientmachine.LoadMachineName()
		if err != nil {
			return fmt.Errorf("could not determine current machine: %w\nRun 'tfw machine init <name>' first", err)
		}

		watcherSvc := pb.NewFileWatcherServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var filterRequests []*pb.AddFilterRequest = make([]*pb.AddFilterRequest, 0, len(filters))
		if len(filters) > 0 {
			for _, f := range filters {
				parts := strings.SplitN(f, ":", 3)
				if len(parts) != 3 {
					return fmt.Errorf("invalid filter format: %q", f)
				}
				filterRequest := &pb.AddFilterRequest{
					WatcherName: args[0],
					RuleType:    parts[0],
					PatternType: parts[1],
					Pattern:     parts[2],
				}
				filterRequests = append(filterRequests, filterRequest)
			}
		}

		w, err := watcherSvc.CreateWatcher(ctx, &pb.CreateWatcherRequest{
			Name:          args[0],
			SourcePath:    path,
			FlushExisting: flushExisting,
			Filters:       filterRequests,
			MachineName:   machineName,
		})
		if err != nil {
			return err
		}

		fmt.Println("Watcher created:")
		printWatchers([]*pb.Watcher{w})
		return nil
	},
}

var updateWatcherCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update a watcher's name or path",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		newName, _ := cmd.Flags().GetString("name")
		newPath, _ := cmd.Flags().GetString("path")

		if newName == "" && newPath == "" {
			return fmt.Errorf("at least one of --name or --path must be provided")
		}

		svc := pb.NewFileWatcherServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Resolve name → id (proto UpdateWatcher takes an int64 id)
		existing, err := svc.GetWatcherByName(ctx, &pb.GetWatcherByNameRequest{Name: args[0]})
		if err != nil {
			return fmt.Errorf("watcher %q not found: %w", args[0], err)
		}

		req := &pb.UpdateWatcherRequest{Id: existing.Id}
		if newName != "" {
			req.Name = &newName
		}
		if newPath != "" {
			req.SourcePath = &newPath
		}

		w, err := svc.UpdateWatcher(ctx, req)
		if err != nil {
			return err
		}

		fmt.Println("Watcher updated:")
		printWatchers([]*pb.Watcher{w})
		return nil
	},
}

var deleteWatcherCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a watcher by name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewFileWatcherServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := svc.DeleteWatcher(ctx, &pb.DeleteWatcherRequest{Name: args[0]})
		if err != nil {
			return err
		}

		if resp.Success {
			fmt.Printf("Watcher %q deleted.\n", args[0])
		} else {
			fmt.Printf("Watcher %q could not be deleted.\n", args[0])
		}
		return nil
	},
}

func printWatchedFiles(files []*pb.WatchedFile, showPath bool) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if showPath {
		fmt.Fprintln(w, "FILE PATH\tFLUSHED\tDETECTED AT")
		fmt.Fprintln(w, "---------\t-------\t-----------")
	} else {
		fmt.Fprintln(w, "FILE NAME\tFLUSHED\tDETECTED AT")
		fmt.Fprintln(w, "---------\t-------\t-----------")
	}
	for _, f := range files {
		detected := "-"
		if f.DetectedAt != nil {
			detected = f.DetectedAt.AsTime().Format(time.DateTime)
		}
		// get file name from file path
		if showPath {
			fmt.Fprintf(w, "%s\t%t\t%s\n",
				f.FilePath,
				f.Flushed,
				detected,
			)
			continue
		}
		fileName := f.FilePath
		if idx := strings.LastIndex(f.FilePath, string(os.PathSeparator)); idx != -1 {
			fileName = f.FilePath[idx+1:]
		}
		fmt.Fprintf(w, "%s\t%t\t%s\n",
			fileName,
			f.Flushed,
			detected,
		)
	}
	w.Flush()
}

func printWatchers(watchers []*pb.Watcher) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tMACHINE\tSOURCE PATH\tCREATED AT")
	fmt.Fprintln(w, "--\t----\t-------\t-----------\t----------")
	for _, watcher := range watchers {
		created := "-"
		if watcher.CreatedAt != nil {
			created = watcher.CreatedAt.AsTime().Format(time.DateTime)
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
			watcher.Id,
			watcher.Name,
			watcher.MachineName,
			watcher.SourcePath,
			created,
		)
	}
	w.Flush()
}
