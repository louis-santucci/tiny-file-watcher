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

	m, err := db.CreateMachine("my-machine", "tok-abc")

	require.NoError(t, err)
	assert.Positive(t, m.ID)
	assert.Equal(t, "my-machine", m.Name)
	assert.Equal(t, "tok-abc", m.Token)
	assert.False(t, m.CreatedAt.IsZero())
	assert.False(t, m.UpdatedAt.IsZero())
}

func TestMachineDB_CreateMachine_Upsert(t *testing.T) {
	db := newMachineDB(t)

	first, err := db.CreateMachine("original-name", "tok-upsert")
	require.NoError(t, err)

	// Same token, different name — should update name and updated_at.
	updated, err := db.CreateMachine("new-name", "tok-upsert")

	require.NoError(t, err)
	assert.Equal(t, first.ID, updated.ID)
	assert.Equal(t, "new-name", updated.Name)
	assert.Equal(t, "tok-upsert", updated.Token)
}

// ── GetMachineByName ──────────────────────────────────────────────────────────

func TestMachineDB_GetMachineByName_OK(t *testing.T) {
	db := newMachineDB(t)

	_, err := db.CreateMachine("lookup-machine", "tok-lookup")
	require.NoError(t, err)

	m, err := db.GetMachineByName("lookup-machine")

	require.NoError(t, err)
	assert.Equal(t, "lookup-machine", m.Name)
	assert.Equal(t, "tok-lookup", m.Token)
}

func TestMachineDB_GetMachineByName_NotFound(t *testing.T) {
	db := newMachineDB(t)

	_, err := db.GetMachineByName("ghost")

	assert.ErrorIs(t, err, sql.ErrNoRows)
}

// ── GetMachineByToken ─────────────────────────────────────────────────────────

func TestMachineDB_GetMachineByToken_OK(t *testing.T) {
	db := newMachineDB(t)

	_, err := db.CreateMachine("token-machine", "tok-xyz")
	require.NoError(t, err)

	m, err := db.GetMachineByToken("tok-xyz")

	require.NoError(t, err)
	assert.Equal(t, "token-machine", m.Name)
	assert.Equal(t, "tok-xyz", m.Token)
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

func TestMachineDB_ListMachines_OrderedByName(t *testing.T) {
	db := newMachineDB(t)

	_, err := db.CreateMachine("zebra", "tok-z")
	require.NoError(t, err)
	_, err = db.CreateMachine("alpha", "tok-a")
	require.NoError(t, err)
	_, err = db.CreateMachine("mango", "tok-m")
	require.NoError(t, err)

	machines, err := db.ListMachines()

	require.NoError(t, err)
	require.Len(t, machines, 3)
	assert.Equal(t, "alpha", machines[0].Name)
	assert.Equal(t, "mango", machines[1].Name)
	assert.Equal(t, "zebra", machines[2].Name)
}

// ── DeleteMachine ─────────────────────────────────────────────────────────────

func TestMachineDB_DeleteMachine_OK(t *testing.T) {
	db := newMachineDB(t)

	_, err := db.CreateMachine("delete-me", "tok-del")
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
