package internal

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	DirName              = ".tfw"
	ServerConfigFileName = "tfws.yml"
	ClientConfigFileName = "tfw.yml"
	DatabaseFileName     = "tfw.db"
	TokensFileName       = "tokens.json"
	MachineFileName      = "machine.json"
)

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, DirName), nil
}

func ServerConfigPath() string {
	dir, err := Dir()
	if err != nil {
		panic(fmt.Sprintf("failed to determine server config path: %v", err))
	}
	return filepath.Join(dir, ServerConfigFileName)
}

func ClientConfigPath() string {
	dir, err := Dir()
	if err != nil {
		panic(fmt.Sprintf("failed to determine client config path: %v", err))
	}
	return filepath.Join(dir, ClientConfigFileName)
}

func DatabasePath() string {
	dir, err := Dir()
	if err != nil {
		panic(fmt.Sprintf("failed to determine database path: %v", err))
	}
	return filepath.Join(dir, DatabaseFileName)
}

func TokensPath() string {
	dir, err := Dir()
	if err != nil {
		panic(fmt.Sprintf("failed to determine tokens path: %v", err))
	}
	return filepath.Join(dir, TokensFileName)
}

func MachinePath() string {
	dir, err := Dir()
	if err != nil {
		panic(fmt.Sprintf("failed to determine machine path: %v", err))
	}
	return filepath.Join(dir, MachineFileName)
}
