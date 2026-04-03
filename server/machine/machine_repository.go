package machine

import "tiny-file-watcher/server/database"

// MachineRepository defines persistence operations for Machine entities.
type MachineRepository interface {
	CreateMachine(name string, token string, ip string, sshPort int32, sshUser string, sshPrivateKey string) (*database.Machine, error)
	GetMachineByName(name string) (*database.Machine, error)
	GetMachineByToken(token string) (*database.Machine, error)
	ListMachines() ([]*database.Machine, error)
	DeleteMachine(name string) error
}

// Compile-time assertion: *database.DB must satisfy MachineRepository.
var _ MachineRepository = (*database.DB)(nil)
