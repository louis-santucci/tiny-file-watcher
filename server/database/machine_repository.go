package database

// MachineRepository defines persistence operations for Machine entities.
type MachineRepository interface {
	CreateMachine(name string, token string, ip string, sshPort int32, sshUser string, sshKeyName string, sshHostPublicKeyPath string) (*Machine, error)
	GetMachineByName(name string) (*Machine, error)
	GetMachineByToken(token string) (*Machine, error)
	ListMachines() ([]*Machine, error)
	DeleteMachine(name string) error
}

// Compile-time assertion: *database.DB must satisfy MachineRepository.
var _ MachineRepository = (*DB)(nil)
