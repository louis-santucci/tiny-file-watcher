package machine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"tiny-file-watcher/internal"
)

// ErrNotInitialized is returned when the machine has not been created yet.
var ErrNotInitialized = errors.New("machine not initialized: run 'tfw machine create <name>' first")

type machineState struct {
	Name  string `json:"name"`
	Token string `json:"token"`
}

var machinePath = internal.MachinePath

// SaveMachineState persists the local machine name and token to ~/.tfw/machine.json.
func SaveMachineState(name, token string) error {
	path := machinePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.Marshal(machineState{Name: name, Token: token})
	if err != nil {
		return fmt.Errorf("marshal machine state: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// LoadMachineName reads the locally stored machine name.
// Returns ErrNotInitialized if the file is absent or the name is empty.
func LoadMachineName() (string, error) {
	state, err := loadState()
	if err != nil {
		return "", err
	}
	if state.Name == "" {
		return "", ErrNotInitialized
	}
	return state.Name, nil
}

// LoadMachineToken reads the locally stored machine token.
// Returns ErrNotInitialized if the file is absent or the token is empty.
func LoadMachineToken() (string, error) {
	state, err := loadState()
	if err != nil {
		return "", err
	}
	if state.Token == "" {
		return "", ErrNotInitialized
	}
	return state.Token, nil
}

func loadState() (*machineState, error) {
	data, err := os.ReadFile(machinePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotInitialized
		}
		return nil, fmt.Errorf("read machine state: %w", err)
	}
	var state machineState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse machine state: %w", err)
	}
	return &state, nil
}
