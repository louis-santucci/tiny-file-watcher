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

var redirectionCmd = &cobra.Command{
	Use:     "redirection",
	Aliases: []string{"r"},
	Short:   "Manage file redirections",
}

func init() {
	// create
	createRedirectionCmd.Flags().StringP("target", "t", "", "Target path for the redirection (required)")
	createRedirectionCmd.Flags().Bool("auto-flush", false, "Enable auto-flush on redirection")
	_ = createRedirectionCmd.MarkFlagRequired("target")

	// update
	updateRedirectionCmd.Flags().String("target", "", "New target path for the redirection")
	updateRedirectionCmd.Flags().String("auto-flush", "", "Enable or disable auto-flush (true/false)")

	redirectionCmd.AddCommand(
		getRedirectionCmd,
		createRedirectionCmd,
		updateRedirectionCmd,
		deleteRedirectionCmd,
	)
}

var getRedirectionCmd = &cobra.Command{
	Use:   "get <watcher-name>",
	Short: "Get a file redirection by watcher name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewFileRedirectionServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		r, err := svc.GetFileRedirection(ctx, &pb.GetFileRedirectionRequest{Name: args[0]})
		if err != nil {
			return err
		}

		printRedirections([]*pb.FileRedirection{r})
		return nil
	},
}

var createRedirectionCmd = &cobra.Command{
	Use:   "create <watcher-name>",
	Short: "Create a new file redirection for a watcher",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target, _ := cmd.Flags().GetString("target")
		autoFlush, _ := cmd.Flags().GetBool("auto-flush")

		svc := pb.NewFileRedirectionServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		r, err := svc.CreateFileRedirection(ctx, &pb.CreateFileRedirectionRequest{
			WatcherName: args[0],
			TargetPath:  target,
			AutoFlush:   autoFlush,
		})
		if err != nil {
			return err
		}

		fmt.Println("Redirection created:")
		printRedirections([]*pb.FileRedirection{r})
		return nil
	},
}

var updateRedirectionCmd = &cobra.Command{
	Use:   "update <watcher-name>",
	Short: "Update a file redirection's target path or auto-flush setting",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		newTarget, _ := cmd.Flags().GetString("target")
		autoFlushStr, _ := cmd.Flags().GetString("auto-flush")

		if newTarget == "" && autoFlushStr == "" {
			return fmt.Errorf("at least one of --target or --auto-flush must be provided")
		}

		req := &pb.UpdateFileRedirectionRequest{WatcherName: args[0]}
		if newTarget != "" {
			req.TargetPath = &newTarget
		}
		if autoFlushStr != "" {
			switch autoFlushStr {
			case "true":
				v := true
				req.AutoFlush = &v
			case "false":
				v := false
				req.AutoFlush = &v
			default:
				return fmt.Errorf("--auto-flush must be true or false")
			}
		}

		svc := pb.NewFileRedirectionServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		r, err := svc.UpdateFileRedirection(ctx, req)
		if err != nil {
			return err
		}

		fmt.Println("Redirection updated:")
		printRedirections([]*pb.FileRedirection{r})
		return nil
	},
}

var deleteRedirectionCmd = &cobra.Command{
	Use:   "delete <watcher-name>",
	Short: "Delete a file redirection by watcher name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewFileRedirectionServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := svc.DeleteFileRedirection(ctx, &pb.DeleteFileRedirectionRequest{WatcherName: args[0]})
		if err != nil {
			return err
		}

		if resp.Success {
			fmt.Printf("Redirection for watcher %q deleted.\n", args[0])
		} else {
			fmt.Printf("Redirection for watcher %q could not be deleted.\n", args[0])
		}
		return nil
	},
}

func printRedirections(redirections []*pb.FileRedirection) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "WATCHER\tTARGET PATH\tAUTO FLUSH\tCREATED AT")
	fmt.Fprintln(w, "-------\t-----------\t----------\t----------")
	for _, r := range redirections {
		created := "-"
		if r.CreatedAt != nil {
			created = r.CreatedAt.AsTime().Format(time.DateTime)
		}
		fmt.Fprintf(w, "%s\t%s\t%v\t%s\n",
			r.WatcherName,
			r.TargetPath,
			r.AutoFlush,
			created,
		)
	}
	w.Flush()
}
