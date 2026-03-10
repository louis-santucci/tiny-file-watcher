package cmd

import (
	"fmt"
	"os"

	"github.com/ridgelines/go-config"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultConfigPath = "/Users/louissantucci/.tfw/tfw.yml"
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
}

func dial() error {
	addr, err := readGRPCAddress()
	if err != nil {
		return fmt.Errorf("could not read server address from config: %w", err)
	}
	c, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("could not connect to %s: %w", addr, err)
	}
	conn = c
	return nil
}

func readGRPCAddress() (string, error) {
	yamlFile := config.NewYAMLFile(defaultConfigPath)
	loader := config.NewOnceLoader(yamlFile)
	cfg := config.NewConfig([]config.Provider{loader})
	if err := cfg.Load(); err != nil {
		return "", err
	}
	addr, err := cfg.String("grpc.address")
	if err != nil || addr == "" {
		return "", fmt.Errorf("grpc.address not set in %s", defaultConfigPath)
	}
	return addr, nil
}
