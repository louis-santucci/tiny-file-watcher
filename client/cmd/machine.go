package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	pb "tiny-file-watcher/gen/grpc"
)

var machineCmd = &cobra.Command{
	Use:     "machine",
	Aliases: []string{"m"},
	Short:   "Manage machines",
}

var (
	machineIP         string
	machineSSHPort    int32
	machineSSHUser    string
	machineSSHKeyPath string
)

func init() {
	createMachineCmd.Flags().StringVar(&machineIP, "ip", "", "IP address of the machine (required)")
	createMachineCmd.Flags().Int32Var(&machineSSHPort, "ssh-port", 22, "SSH port of the machine")
	createMachineCmd.Flags().StringVar(&machineSSHUser, "ssh-user", "", "SSH user for the machine (required)")
	createMachineCmd.Flags().StringVar(&machineSSHKeyPath, "ssh-key", "", "Full path to the SSH private key file on this machine, e.g. /home/user/.ssh/id_ed25519 (required)")
	_ = createMachineCmd.MarkFlagRequired("ip")
	_ = createMachineCmd.MarkFlagRequired("ssh-user")
	_ = createMachineCmd.MarkFlagRequired("ssh-key")
	updateMachineCmd.Flags().StringVar(&machineIP, "ip", "", "New IP address of the machine")
	updateMachineCmd.Flags().Int32Var(&machineSSHPort, "ssh-port", 0, "New SSH port of the machine")
	updateMachineCmd.Flags().StringVar(&machineSSHUser, "ssh-user", "", "New SSH user for the machine")
	updateMachineCmd.Flags().StringVar(&machineSSHKeyPath, "ssh-key", "", "New full path to the SSH private key file on this machine, e.g. /home/user/.ssh/id_ed25519")

	machineCmd.AddCommand(createMachineCmd, listMachinesCmd, deleteMachineCmd, updateMachineCmd)
}

var updateMachineCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update an existing machine",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := pb.NewMachineServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		fmt.Printf("Updating machine %s\n", args[0])

		var ip, sshUser, sshKeyPath *string = nil, nil, nil
		var sshPort *int32 = nil

		if machineIP != "" {
			ip = &machineIP
		}
		if machineSSHPort != 0 {
			sshPort = &machineSSHPort
		}
		if machineSSHUser != "" {
			sshUser = &machineSSHUser
		}
		if machineSSHKeyPath != "" {
			sshKeyPath = &machineSSHKeyPath
		}

		resp, err := svc.UpdateMachine(ctx, &pb.UpdateMachineRequest{
			Name:          args[0],
			Ip:            ip,
			SshUser:       sshUser,
			SshPrivateKey: sshKeyPath,
			SshPort:       sshPort,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Machine %q updated (ip: %s, ssh-port: %d, ssh-user: %s)\n", resp.Name, resp.Ip, resp.SshPort, resp.SshUser)
		return nil
	},
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

		fmt.Printf("Creating machine %q with IP %q, SSH port %d, SSH user %q...\n", args[0], machineIP, machineSSHPort, machineSSHUser)
		fmt.Printf("Using SSH key %q\n", machineSSHKeyPath)
		resp, err := svc.CreateMachine(ctx, &pb.InitializeMachineRequest{
			Name:          args[0],
			Ip:            machineIP,
			SshPort:       machineSSHPort,
			SshUser:       machineSSHUser,
			SshPrivateKey: machineSSHKeyPath,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Machine %q created (ip: %s, ssh-port: %d, ssh-user: %s)\n", resp.Name, resp.Ip, resp.SshPort, machineSSHUser)
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
