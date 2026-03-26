package internal

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	DirName          = ".tfw"
	ConfigFileName   = "tfw.yml"
	DatabaseFileName = "tfw.db"
	TokensFileName   = "tokens.json"
)

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, DirName), nil
}

func ConfigPath() string {
	dir, err := Dir()
	if err != nil {
		panic(fmt.Sprintf("failed to determine config path: %v", err))
	}
	return filepath.Join(dir, ConfigFileName)
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
