package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
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
	createMachineIP             string
	createMachineSSHPort        int32
	createMachineSSHUser        string
	createMachineSSHKeyPath     string
	createMachineSSHHostKeyPath string
)

func init() {
	createMachineCmd.Flags().StringVar(&createMachineIP, "ip", "", "IP address of the machine (required)")
	createMachineCmd.Flags().Int32Var(&createMachineSSHPort, "ssh-port", 22, "SSH port of the machine")
	createMachineCmd.Flags().StringVar(&createMachineSSHUser, "ssh-user", "", "SSH user for the machine (required)")
	createMachineCmd.Flags().StringVar(&createMachineSSHKeyPath, "ssh-key", "", "Path to the SSH private key file (required)")
	createMachineCmd.Flags().StringVar(&createMachineSSHHostKeyPath, "ssh-host-key", "", "Server-side path to the host public key file in authorized_keys format, e.g. obtained via ssh-keyscan (required)")
	_ = createMachineCmd.MarkFlagRequired("ip")
	_ = createMachineCmd.MarkFlagRequired("ssh-user")
	_ = createMachineCmd.MarkFlagRequired("ssh-key")
	_ = createMachineCmd.MarkFlagRequired("ssh-host-key")

	machineCmd.AddCommand(createMachineCmd, listMachinesCmd, deleteMachineCmd)
}

var createMachineCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Register this machine with the server",
	Long: `Register the current machine under the given name.
A unique token is generated and saved locally to ~/.tfw/machine.json.
Requires authentication (run 'tfw login' first).

The --ssh-key flag must point to a local private key file (e.g. ~/.ssh/id_rsa).
Its content is sent to the server and stored for future SSH connections.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token := uuid.New().String()

		svc := pb.NewMachineServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := svc.CreateMachine(ctx, &pb.InitializeMachineRequest{
			Name:                 args[0],
			Token:                token,
			Ip:                   createMachineIP,
			SshPort:              createMachineSSHPort,
			SshUser:              createMachineSSHUser,
			SshPrivateKey:        createMachineSSHKeyPath,
			SshHostPublicKeyPath: createMachineSSHHostKeyPath,
		})
		if err != nil {
			return err
		}

		if err := clientmachine.SaveMachineState(resp.Name, token); err != nil {
			return fmt.Errorf("save machine state locally: %w", err)
		}

		fmt.Printf("Machine %q created (token: %s, ip: %s, ssh-port: %d, ssh-user: %s)\n", resp.Name, token, resp.Ip, resp.SshPort, createMachineSSHUser)
		fmt.Printf("Machine state saved to ~/.tfw/machine.json\n")
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
	fmt.Fprintln(w, "NAME\tIP\tSSH PORT\tSSH USER\tSSH PRIVATE KEY\tSSH HOST PUBLIC KEY PATH\tTOKEN\tCREATED AT")
	fmt.Fprintln(w, "----\t--\t--------\t--------\t---------------\t------------------------\t-----\t----------")
	for _, m := range machines {
		created := "-"
		if m.CreatedAt != nil {
			created = m.CreatedAt.AsTime().Format(time.DateTime)
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n", m.Name, m.Ip, m.SshPort, m.SshUser, m.SshPrivateKey, m.SshHostPublicKeyPath, m.Token, created)
	}
	w.Flush()
}
