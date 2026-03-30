package mocks

import (
	"tiny-file-watcher/server/database"

	"github.com/stretchr/testify/mock"
)

// MockMachineRepository is a testify mock for machine.MachineRepository.
type MockMachineRepository struct {
	mock.Mock
}

func (m *MockMachineRepository) CreateMachine(name, token string) (*database.Machine, error) {
	args := m.Called(name, token)
	if v := args.Get(0); v != nil {
		return v.(*database.Machine), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockMachineRepository) GetMachineByName(name string) (*database.Machine, error) {
	args := m.Called(name)
	if v := args.Get(0); v != nil {
		return v.(*database.Machine), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockMachineRepository) GetMachineByToken(token string) (*database.Machine, error) {
	args := m.Called(token)
	if v := args.Get(0); v != nil {
		return v.(*database.Machine), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockMachineRepository) ListMachines() ([]*database.Machine, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.([]*database.Machine), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockMachineRepository) DeleteMachine(name string) error {
	args := m.Called(name)
	return args.Error(0)
}
