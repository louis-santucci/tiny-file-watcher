package database

import (
	"fmt"
	"strings"
	"time"
)

// Machine mirrors the machines table row.
type Machine struct {
	ID                int64
	Name              string
	IP                string
	SSHPort           int32
	SSHUser           string
	SSHPrivateKeyPath string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (db *DB) CreateMachine(name, ip string, sshPort int32, sshUser string, sshPrivateKeyPath string) (*Machine, error) {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	_, err := db.conn.Exec(
		`INSERT INTO machines (name, ip, ssh_port, ssh_user, ssh_private_key_path, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		name, ip, sshPort, sshUser, sshPrivateKeyPath, nowStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("create machine: %w", err)
	}
	return db.GetMachineByName(name)
}

// GetMachineByName returns a Machine by its unique name.
func (db *DB) GetMachineByName(name string) (*Machine, error) {
	row := db.conn.QueryRow(
		`SELECT id, name, ip, ssh_port, ssh_user, ssh_private_key_path, created_at, updated_at FROM machines WHERE name = ?`, name,
	)
	return scanMachine(row)
}

// ListMachines returns all registered machines.
func (db *DB) ListMachines() ([]*Machine, error) {
	rows, err := db.conn.Query(
		`SELECT id, name, ip, ssh_port, ssh_user, ssh_private_key_path, created_at, updated_at FROM machines ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list machines: %w", err)
	}
	defer rows.Close()
	var result []*Machine
	for rows.Next() {
		m, err := scanMachine(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// DeleteMachine deletes a machine by name.
func (db *DB) DeleteMachine(name string) error {
	_, err := db.conn.Exec(`DELETE FROM machines WHERE name = ?`, name)
	return err
}

// UpdateMachine updates an existing machine. If all parameters are nil, return the existing machine, else update the machine in db and returns the updated machine.
func (db *DB) UpdateMachine(name string, ip *string, sshPort *int32, sshUser *string, sshPrivateKeyPath *string) (*Machine, error) {
	if ip == nil && sshPort == nil && sshUser == nil && sshPrivateKeyPath == nil {
		return db.GetMachineByName(name)
	}

	now := time.Now().UTC()
	setClauses := make([]string, 0)
	args := []any{}

	if ip != nil {
		setClauses = append(setClauses, "ip=?")
		args = append(args, *ip)
	}
	if sshPort != nil {
		setClauses = append(setClauses, "ssh_port=?")
		args = append(args, *sshPort)
	}
	if sshPrivateKeyPath != nil {
		setClauses = append(setClauses, "ssh_private_key_path=?")
		args = append(args, *sshPrivateKeyPath)
	}
	if sshUser != nil {
		setClauses = append(setClauses, "ssh_user=?")
		args = append(args, *sshUser)
	}
	if sshPrivateKeyPath != nil {
		setClauses = append(setClauses, "ssh_private_key_path=?")
		args = append(args, *sshPrivateKeyPath)
	}
	setClauses = append(setClauses, "updated_at=?")
	args = append(args, now.Format(time.RFC3339))
	args = append(args, name) // WHERE name=?

	query := fmt.Sprintf("UPDATE machines SET %s WHERE name=?", strings.Join(setClauses, ", "))
	res, err := db.conn.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("update machine: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("machine not found")
	}
	return db.GetMachineByName(name)
}

func scanMachine(s scanner) (*Machine, error) {
	var m Machine
	var createdStr, updatedStr string
	if err := s.Scan(&m.ID, &m.Name, &m.IP, &m.SSHPort, &m.SSHUser, &m.SSHPrivateKeyPath, &createdStr, &updatedStr); err != nil {
		return nil, fmt.Errorf("scan machine: %w", err)
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	return &m, nil
}
