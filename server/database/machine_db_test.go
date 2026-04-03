//go:build integration

package database_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"tiny-file-watcher/server/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMachineDB(t *testing.T) *database.DB {
	t.Helper()
	path := "file:" + filepath.Join(t.TempDir(), "test.db") +
		"?_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := database.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

// ── CreateMachine ─────────────────────────────────────────────────────────────

func TestMachineDB_CreateMachine_OK(t *testing.T) {
	db := newMachineDB(t)

	m, err := db.CreateMachine("my-machine", "tok-abc", "192.168.1.1", 22, "ssh-user", "ssh-key")

	require.NoError(t, err)
	assert.Positive(t, m.ID)
	assert.Equal(t, "my-machine", m.Name)
	assert.Equal(t, "tok-abc", m.Token)
	assert.Equal(t, "192.168.1.1", m.IP)
	assert.Equal(t, int32(22), m.SSHPort)
	assert.Equal(t, "ssh-user", m.SSHUser)
	assert.Equal(t, "ssh-user", m.SSHUser)
	assert.False(t, m.CreatedAt.IsZero())
	assert.False(t, m.UpdatedAt.IsZero())
}

func TestMachineDB_CreateMachine_ErrorDuplicateToken(t *testing.T) {
	db := newMachineDB(t)

	_, err := db.CreateMachine("my-machine", "tok-abc", "192.168.1.1", 22, "ssh-user", "ssh-key")
	require.NoError(t, err)
	_, err = db.CreateMachine("my-machine_2", "tok-abc", "192.168.1.2", 22, "ssh-user", "ssh-key")
	if assert.Error(t, err) {
		assert.ErrorContains(t, err, "UNIQUE constraint failed: machines.token")
	}
	// ── GetMachineByName ──────────────────────────────────────────────────────────
}

func TestMachineDB_GetMachineByName_OK(t *testing.T) {
	db := newMachineDB(t)

	_, err := db.CreateMachine("lookup-machine", "tok-lookup", "172.16.0.1", 22, "ssh-user", "ssh-key")
	require.NoError(t, err)

	m, err := db.GetMachineByName("lookup-machine")

	require.NoError(t, err)
	assert.Equal(t, "lookup-machine", m.Name)
	assert.Equal(t, "tok-lookup", m.Token)
	assert.Equal(t, "172.16.0.1", m.IP)
	assert.Equal(t, int32(22), m.SSHPort)
	assert.Equal(t, "ssh-user", m.SSHUser)
	assert.Equal(t, "ssh-key", m.SSHKeyName)
}

func TestMachineDB_GetMachineByName_NotFound(t *testing.T) {
	db := newMachineDB(t)

	_, err := db.GetMachineByName("ghost")

	assert.ErrorIs(t, err, sql.ErrNoRows)
}

// ── GetMachineByToken ─────────────────────────────────────────────────────────

func TestMachineDB_GetMachineByToken_OK(t *testing.T) {
	db := newMachineDB(t)

	_, err := db.CreateMachine("token-machine", "tok-xyz", "192.168.0.10", 22, "ssh-user", "ssh-key")
	require.NoError(t, err)

	m, err := db.GetMachineByToken("tok-xyz")

	require.NoError(t, err)
	assert.Equal(t, "token-machine", m.Name)
	assert.Equal(t, "tok-xyz", m.Token)
	assert.Equal(t, "192.168.0.10", m.IP)
	assert.Equal(t, int32(22), m.SSHPort)
	assert.Equal(t, "ssh-user", m.SSHUser)
	assert.Equal(t, "ssh-key", m.SSHKeyName)
}

func TestMachineDB_GetMachineByToken_NotFound(t *testing.T) {
	db := newMachineDB(t)

	_, err := db.GetMachineByToken("no-such-token")

	assert.ErrorIs(t, err, sql.ErrNoRows)
}

// ── ListMachines ──────────────────────────────────────────────────────────────

func TestMachineDB_ListMachines_Empty(t *testing.T) {
	db := newMachineDB(t)

	machines, err := db.ListMachines()

	require.NoError(t, err)
	assert.Empty(t, machines)
}

// ── DeleteMachine ─────────────────────────────────────────────────────────────

func TestMachineDB_DeleteMachine_OK(t *testing.T) {
	db := newMachineDB(t)

	_, err := db.CreateMachine("delete-me", "tok-del", "1.1.1.1", 22, "ssh-user", "ssh-key")
	require.NoError(t, err)

	err = db.DeleteMachine("delete-me")
	require.NoError(t, err)

	_, err = db.GetMachineByName("delete-me")
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestMachineDB_DeleteMachine_NotFound(t *testing.T) {
	db := newMachineDB(t)

	// Deleting a non-existent machine is a no-op (no error).
	err := db.DeleteMachine("ghost-machine")

	assert.NoError(t, err)
}
