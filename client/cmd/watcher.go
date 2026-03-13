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

var watcherCmd = &cobra.Command{
	Use:     "watcher",
	Aliases: []string{"w"},
	Short:   "Manage file watchers",
}

func init() {
	// create
	createWatcherCmd.Flags().StringP("path", "p", "", "Source path to watch (required)")
	_ = createWatcherCmd.MarkFlagRequired("path")

	// update
	updateWatcherCmd.Flags().String("name", "", "New name for the watcher")
	updateWatcherCmd.Flags().String("path", "", "New source path for the watcher")

	watcherCmd.AddCommand(
		listWatchersCmd,
		getWatcherCmd,
		createWatcherCmd,
		updateWatcherCmd,
		deleteWatcherCmd,
		toggleWatcherCmd,
	)
}

var listWatchersCmd = &cobra.Command{
	Use:   "list",
	Short: "List all watchers",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewFileWatcherServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := svc.ListWatchers(ctx, &pb.ListWatchersRequest{})
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
		svc := pb.NewFileWatcherServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		w, err := svc.GetWatcherByName(ctx, &pb.GetWatcherByNameRequest{Name: args[0]})
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

		svc := pb.NewFileWatcherServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		w, err := svc.CreateWatcher(ctx, &pb.CreateWatcherRequest{
			Name:       args[0],
			SourcePath: path,
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

var toggleWatcherCmd = &cobra.Command{
	Use:   "toggle <name>",
	Short: "Enable or disable a watcher",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewFileWatcherServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		w, err := svc.ToggleWatcher(ctx, &pb.ToggleWatcherRequest{Name: args[0]})
		if err != nil {
			return err
		}

		state := "disabled"
		if w.Enabled {
			state = "enabled"
		}
		fmt.Printf("Watcher %q is now %s.\n", w.Name, state)
		printWatchers([]*pb.Watcher{w})
		return nil
	},
}

func printWatchers(watchers []*pb.Watcher) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSOURCE PATH\tENABLED\tCREATED AT")
	fmt.Fprintln(w, "--\t----\t-----------\t-------\t----------")
	for _, watcher := range watchers {
		created := "-"
		if watcher.CreatedAt != nil {
			created = watcher.CreatedAt.AsTime().Format(time.DateTime)
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%v\t%s\n",
			watcher.Id,
			watcher.Name,
			watcher.SourcePath,
			watcher.Enabled,
			created,
		)
	}
	w.Flush()
}
