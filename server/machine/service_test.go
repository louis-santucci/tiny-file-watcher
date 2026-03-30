package machine_test

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "tiny-file-watcher/gen/grpc"
	"tiny-file-watcher/server/database"
	"tiny-file-watcher/server/machine"
	"tiny-file-watcher/server/test/mocks"
	"tiny-file-watcher/server/test/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ctx     = context.Background()
	fixedAt = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
)

func newMachine(id int64, name, token string) *database.Machine {
	return &database.Machine{
		ID:        id,
		Name:      name,
		Token:     token,
		CreatedAt: fixedAt,
		UpdatedAt: fixedAt,
	}
}

func newService(repo *mocks.MockMachineRepository) *machine.MachineService {
	return machine.NewMachineService(repo, testutil.TestLogger())
}

func assertCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	s, ok := status.FromError(err)
	assert.True(t, ok, "expected a gRPC status error")
	assert.Equal(t, want, s.Code())
}

// ── CreateMachine ─────────────────────────────────────────────────────────────

func TestCreateMachine_OK(t *testing.T) {
	repo := &mocks.MockMachineRepository{}
	svc := newService(repo)

	m := newMachine(1, "my-machine", "tok-abc")
	repo.On("CreateMachine", "my-machine", "tok-abc").Return(m, nil)

	resp, err := svc.CreateMachine(ctx, &pb.InitializeMachineRequest{Name: "my-machine", Token: "tok-abc"})

	assert.NoError(t, err)
	assert.Equal(t, "my-machine", resp.Name)
	assert.Equal(t, "tok-abc", resp.Token)
	assert.Equal(t, fixedAt.Unix(), resp.CreatedAt.AsTime().Unix())
	assert.Equal(t, fixedAt.Unix(), resp.UpdatedAt.AsTime().Unix())
	repo.AssertExpectations(t)
}

func TestCreateMachine_MissingName(t *testing.T) {
	svc := newService(&mocks.MockMachineRepository{})

	_, err := svc.CreateMachine(ctx, &pb.InitializeMachineRequest{Name: "", Token: "tok-abc"})

	assertCode(t, err, codes.InvalidArgument)
}

func TestCreateMachine_MissingToken(t *testing.T) {
	svc := newService(&mocks.MockMachineRepository{})

	_, err := svc.CreateMachine(ctx, &pb.InitializeMachineRequest{Name: "my-machine", Token: ""})

	assertCode(t, err, codes.InvalidArgument)
}

func TestCreateMachine_DBError(t *testing.T) {
	repo := &mocks.MockMachineRepository{}
	svc := newService(repo)

	repo.On("CreateMachine", "my-machine", "tok-abc").Return(nil, errors.New("db error"))

	_, err := svc.CreateMachine(ctx, &pb.InitializeMachineRequest{Name: "my-machine", Token: "tok-abc"})

	assertCode(t, err, codes.Internal)
	repo.AssertExpectations(t)
}

// ── GetMachines ───────────────────────────────────────────────────────────────

func TestGetMachines_Empty(t *testing.T) {
	repo := &mocks.MockMachineRepository{}
	svc := newService(repo)

	repo.On("ListMachines").Return([]*database.Machine{}, nil)

	resp, err := svc.GetMachines(ctx, &pb.EmptyRequest{})

	assert.NoError(t, err)
	assert.Empty(t, resp.Machines)
	repo.AssertExpectations(t)
}

func TestGetMachines_Multiple(t *testing.T) {
	repo := &mocks.MockMachineRepository{}
	svc := newService(repo)

	machines := []*database.Machine{
		newMachine(1, "machine-a", "tok-a"),
		newMachine(2, "machine-b", "tok-b"),
	}
	repo.On("ListMachines").Return(machines, nil)

	resp, err := svc.GetMachines(ctx, &pb.EmptyRequest{})

	assert.NoError(t, err)
	assert.Len(t, resp.Machines, 2)
	assert.Equal(t, "machine-a", resp.Machines[0].Name)
	assert.Equal(t, "tok-a", resp.Machines[0].Token)
	assert.Equal(t, "machine-b", resp.Machines[1].Name)
	assert.Equal(t, "tok-b", resp.Machines[1].Token)
	repo.AssertExpectations(t)
}

func TestGetMachines_DBError(t *testing.T) {
	repo := &mocks.MockMachineRepository{}
	svc := newService(repo)

	repo.On("ListMachines").Return(nil, errors.New("db error"))

	_, err := svc.GetMachines(ctx, &pb.EmptyRequest{})

	assertCode(t, err, codes.Internal)
	repo.AssertExpectations(t)
}

// ── DeleteMachine ─────────────────────────────────────────────────────────────

func TestDeleteMachine_OK(t *testing.T) {
	repo := &mocks.MockMachineRepository{}
	svc := newService(repo)

	repo.On("DeleteMachine", "my-machine").Return(nil)

	resp, err := svc.DeleteMachine(ctx, &pb.DeleteMachineRequest{Name: "my-machine"})

	assert.NoError(t, err)
	assert.True(t, resp.Success)
	repo.AssertExpectations(t)
}

func TestDeleteMachine_MissingName(t *testing.T) {
	svc := newService(&mocks.MockMachineRepository{})

	_, err := svc.DeleteMachine(ctx, &pb.DeleteMachineRequest{Name: ""})

	assertCode(t, err, codes.InvalidArgument)
}

func TestDeleteMachine_DBError(t *testing.T) {
	repo := &mocks.MockMachineRepository{}
	svc := newService(repo)

	repo.On("DeleteMachine", "my-machine").Return(errors.New("db error"))

	_, err := svc.DeleteMachine(ctx, &pb.DeleteMachineRequest{Name: "my-machine"})

	assertCode(t, err, codes.Internal)
	repo.AssertExpectations(t)
}

// suppress "imported and not used" for mock package
var _ = mock.Anything
