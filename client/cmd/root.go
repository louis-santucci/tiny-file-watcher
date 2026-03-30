package cmd

import (
	"fmt"
	"os"
	"tiny-file-watcher/client/auth"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var conn *grpc.ClientConn

var rootCmd = &cobra.Command{
	Use:   "tfw",
	Short: "tiny-file-watcher CLI",
	Long:  "tfw is a CLI client for the tiny-file-watcher gRPC server (tfws).",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return dial()
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if conn != nil {
			conn.Close()
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(watcherCmd)
	rootCmd.AddCommand(redirectionCmd)
	rootCmd.AddCommand(flushCmd)
	rootCmd.AddCommand(filterCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(syncWatcherCmd)
	rootCmd.AddCommand(machineCmd)
}

func dial() error {
	cfg, err := loadClientConfig()
	if err != nil {
		return fmt.Errorf("load client config: %w", err)
	}

	var opts []grpc.DialOption

	oidcEnabled, _ := cfg.Bool("oidc.enabled")
	if oidcEnabled {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

		oidcIssuer, _ := cfg.String("oidc.issuer")
		oidcClientID, _ := cfg.String("oidc.device-client-id")

		oidcCredentials, err := auth.NewTokenCredentials(oidcIssuer, oidcClientID)
		if err != nil {
			return err // "not logged in: run 'tfw login' first"
		}
		opts = append(opts, grpc.WithPerRPCCredentials(oidcCredentials))
	}
	addr, _ := cfg.String("grpc.address")

	c, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return fmt.Errorf("could not connect to %s: %w", addr, err)
	}
	conn = c
	return nil
}
