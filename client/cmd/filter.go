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

var filterCmd = &cobra.Command{
	Use:     "filter",
	Aliases: []string{"f"},
	Short:   "Manage watcher file filters",
}

func init() {
	addFilterCmd.Flags().StringP("type", "t", "", "Rule type: include or exclude (required)")
	_ = addFilterCmd.MarkFlagRequired("type")
	addFilterCmd.Flags().StringP("match", "m", "", "Pattern type: extension, name, or glob (required)")
	_ = addFilterCmd.MarkFlagRequired("match")
	addFilterCmd.Flags().StringP("pattern", "p", "", "Pattern value, e.g. .mp3, myfile.txt, *.tmp (required)")
	_ = addFilterCmd.MarkFlagRequired("pattern")

	filterCmd.AddCommand(addFilterCmd, listFiltersCmd, deleteFilterCmd)
}

var addFilterCmd = &cobra.Command{
	Use:   "add <watcher-name>",
	Short: "Add a filter rule to a watcher",
	Long: `Add an inclusion or exclusion rule to a watcher.

Examples:
  tfw filter add my-watcher --type include --match extension --pattern .mp3
  tfw filter add my-watcher --type include --match extension --pattern .wav
  tfw filter add my-watcher --type exclude --match glob --pattern "*.tmp"
  tfw filter add my-watcher --type exclude --match name --pattern ".DS_Store"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ruleType, _ := cmd.Flags().GetString("type")
		patternType, _ := cmd.Flags().GetString("match")
		pattern, _ := cmd.Flags().GetString("pattern")

		svc := pb.NewWatcherFilterServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		f, err := svc.AddFilter(ctx, &pb.AddFilterRequest{
			WatcherName: args[0],
			RuleType:    ruleType,
			PatternType: patternType,
			Pattern:     pattern,
		})
		if err != nil {
			return err
		}

		fmt.Println("Filter added:")
		printFilters([]*pb.WatcherFilter{f})
		return nil
	},
}

var listFiltersCmd = &cobra.Command{
	Use:   "list [watcher-name]",
	Short: "List filters (optionally scoped to a watcher)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := &pb.ListFiltersRequest{}
		if len(args) == 1 {
			req.WatcherName = args[0]
		}

		svc := pb.NewWatcherFilterServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := svc.ListFilters(ctx, req)
		if err != nil {
			return err
		}

		printFilters(resp.Filters)
		return nil
	},
}

var deleteFilterCmd = &cobra.Command{
	Use:   "delete <filter-id>",
	Short: "Delete a filter by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var id int64
		if _, err := fmt.Sscan(args[0], &id); err != nil || id < 1 {
			return fmt.Errorf("filter-id must be a positive integer")
		}

		svc := pb.NewWatcherFilterServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := svc.DeleteFilter(ctx, &pb.DeleteFilterRequest{Id: id})
		if err != nil {
			return err
		}

		if resp.Success {
			fmt.Printf("Filter %d deleted.\n", id)
		} else {
			fmt.Printf("Filter %d could not be deleted.\n", id)
		}
		return nil
	},
}

func printFilters(filters []*pb.WatcherFilter) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tWATCHER\tTYPE\tMATCH\tPATTERN")
	fmt.Fprintln(w, "--\t-------\t----\t-----\t-------")
	for _, f := range filters {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
			f.Id, f.WatcherName, f.RuleType, f.PatternType, f.Pattern)
	}
	w.Flush()
}
