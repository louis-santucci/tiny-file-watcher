package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	clientmachine "tiny-file-watcher/client/machine"
	pb "tiny-file-watcher/gen/grpc"
)

var machineCmd = &cobra.Command{
	Use:     "machine",
	Aliases: []string{"m"},
	Short:   "Manage machines",
}

var (
	createMachineIP         string
	createMachineSSHPort    int32
	createMachineSSHUser    string
	createMachineSSHKeyPath string
	createMachineSet        bool
)

func init() {
	createMachineCmd.Flags().StringVar(&createMachineIP, "ip", "", "IP address of the machine (required)")
	createMachineCmd.Flags().Int32Var(&createMachineSSHPort, "ssh-port", 22, "SSH port of the machine")
	createMachineCmd.Flags().StringVar(&createMachineSSHUser, "ssh-user", "", "SSH user for the machine (required)")
	createMachineCmd.Flags().StringVar(&createMachineSSHKeyPath, "ssh-key", "", "Full path to the SSH private key file on this machine, e.g. /home/user/.ssh/id_ed25519 (required)")
	createMachineCmd.Flags().BoolVar(&createMachineSet, "set", false, "Save the machine name locally after creation (~/.tfw/machine.json)")
	_ = createMachineCmd.MarkFlagRequired("ip")
	_ = createMachineCmd.MarkFlagRequired("ssh-user")
	_ = createMachineCmd.MarkFlagRequired("ssh-key")

	machineCmd.AddCommand(createMachineCmd, listMachinesCmd, deleteMachineCmd, setMachineCmd, unsetMachineCmd)
}

var createMachineCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Register this machine with the server",
	Long: `Register the current machine under the given name.
Requires authentication (run 'tfw login' first).

The --ssh-key flag must be the full path to a private key file on this machine (e.g. /home/user/.ssh/id_ed25519).
The path is stored on the server and used for future SSH connections to this machine.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewMachineServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		fmt.Printf("Creating machine %q with IP %q, SSH port %d, SSH user %q...\n", args[0], createMachineIP, createMachineSSHPort, createMachineSSHUser)
		fmt.Printf("Using SSH key %q\n", createMachineSSHKeyPath)
		resp, err := svc.CreateMachine(ctx, &pb.InitializeMachineRequest{
			Name:          args[0],
			Ip:            createMachineIP,
			SshPort:       createMachineSSHPort,
			SshUser:       createMachineSSHUser,
			SshPrivateKey: createMachineSSHKeyPath,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Machine %q created (ip: %s, ssh-port: %d, ssh-user: %s)\n", resp.Name, resp.Ip, resp.SshPort, createMachineSSHUser)

		if createMachineSet {
			if err := clientmachine.SaveMachineState(resp.Name); err != nil {
				return fmt.Errorf("save machine state locally: %w", err)
			}
			fmt.Printf("Machine state saved to ~/.tfw/machine.json\n")
		}
		return nil
	},
}

var listMachinesCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered machines",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewMachineServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := svc.GetMachines(ctx, &pb.EmptyRequest{})
		if err != nil {
			return err
		}

		printMachines(resp.Machines)
		return nil
	},
}

var deleteMachineCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a machine by name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewMachineServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := svc.DeleteMachine(ctx, &pb.DeleteMachineRequest{Name: args[0]})
		if err != nil {
			return err
		}

		if resp.Success {
			fmt.Printf("Machine %q deleted.\n", args[0])
		} else {
			fmt.Printf("Machine %q could not be deleted.\n", args[0])
		}
		return nil
	},
}

func printMachines(machines []*pb.MachineResponse) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tIP\tSSH PORT\tSSH USER\tSSH PRIVATE KEY\tCREATED AT")
	fmt.Fprintln(w, "----\t--\t--------\t--------\t---------------\t----------")
	for _, m := range machines {
		created := "-"
		if m.CreatedAt != nil {
			created = m.CreatedAt.AsTime().Format(time.DateTime)
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n", m.Name, m.Ip, m.SshPort, m.SshUser, m.SshPrivateKey, created)
	}
	w.Flush()
}

var setMachineCmd = &cobra.Command{
	Use:   "set <name>",
	Short: "Set the local machine name from a registered machine",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewMachineServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := svc.GetMachines(ctx, &pb.EmptyRequest{})
		if err != nil {
			return err
		}

		for _, m := range resp.Machines {
			if m.Name == args[0] {
				if err := clientmachine.SaveMachineState(m.Name); err != nil {
					return fmt.Errorf("save machine state locally: %w", err)
				}
				fmt.Printf("Machine %q set. State saved to ~/.tfw/machine.json\n", m.Name)
				return nil
			}
		}
		return fmt.Errorf("machine %q not found", args[0])
	},
}

var unsetMachineCmd = &cobra.Command{
	Use:   "unset",
	Short: "Remove the locally stored machine state",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := clientmachine.ClearMachineState(); err != nil {
			return err
		}
		fmt.Println("Machine state cleared.")
		return nil
	},
}
