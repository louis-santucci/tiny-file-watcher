package database

import (
	"fmt"
	"time"
)

// Machine mirrors the machines table row.
type Machine struct {
	ID        int64
	Token     string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateMachine upserts a machine record identified by its token.
// If a machine with the given token already exists its name and updated_at
// are refreshed; otherwise a new row is inserted.
func (db *DB) CreateMachine(name, token string) (*Machine, error) {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	_, err := db.conn.Exec(
		`INSERT INTO machines (token, name, created_at, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(token) DO UPDATE SET name = excluded.name, updated_at = excluded.updated_at`,
		token, name, nowStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("create machine: %w", err)
	}
	return db.GetMachineByToken(token)
}

// GetMachineByName returns a Machine by its unique name.
func (db *DB) GetMachineByName(name string) (*Machine, error) {
	row := db.conn.QueryRow(
		`SELECT id, token, name, created_at, updated_at FROM machines WHERE name = ?`, name,
	)
	return scanMachine(row)
}

// GetMachineByToken returns a Machine by its token.
func (db *DB) GetMachineByToken(token string) (*Machine, error) {
	row := db.conn.QueryRow(
		`SELECT id, token, name, created_at, updated_at FROM machines WHERE token = ?`, token,
	)
	return scanMachine(row)
}

// ListMachines returns all registered machines.
func (db *DB) ListMachines() ([]*Machine, error) {
	rows, err := db.conn.Query(
		`SELECT id, token, name, created_at, updated_at FROM machines ORDER BY name`,
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

func scanMachine(s scanner) (*Machine, error) {
	var m Machine
	var createdStr, updatedStr string
	if err := s.Scan(&m.ID, &m.Token, &m.Name, &createdStr, &updatedStr); err != nil {
		return nil, fmt.Errorf("scan machine: %w", err)
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	return &m, nil
}
