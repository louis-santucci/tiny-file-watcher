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
	Name string `json:"name"`
}

var machinePath = internal.MachinePath

// SaveMachineState persists the local machine name to ~/.tfw/machine.json.
func SaveMachineState(name string) error {
	path := machinePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.Marshal(machineState{Name: name})
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

// ClearMachineState removes the locally stored machine state (~/.tfw/machine.json).
// Returns nil if the file does not exist.
func ClearMachineState() error {
	path := machinePath()
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove machine state: %w", err)
	}
	return nil
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
