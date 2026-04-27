package web

import (
	"fmt"
	"net/http"
	"strconv"

	pb "tiny-file-watcher/gen/grpc"
)

type machineListData struct {
	Machines []*pb.MachineResponse
	Error    string
}

type machineDetailData struct {
	Machine  *pb.MachineResponse
	Watchers []*pb.Watcher
}

func (h *Handler) handleMachineList(w http.ResponseWriter, r *http.Request) {
	resp, err := h.machineSvc.GetMachines(r.Context(), &pb.EmptyRequest{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "machines.html", machineListData{Machines: resp.Machines})
}

func (h *Handler) handleMachineDetail(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	resp, err := h.machineSvc.GetMachines(r.Context(), &pb.EmptyRequest{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var machine *pb.MachineResponse
	for _, m := range resp.Machines {
		if m.Name == name {
			machine = m
			break
		}
	}
	if machine == nil {
		http.Error(w, "machine not found", http.StatusNotFound)
		return
	}

	// List watchers linked to this machine.
	watchersResp, err := h.watcherSvc.ListWatchers(r.Context(), &pb.ListWatchersRequest{MachineName: &name})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.render(w, "machine.html", machineDetailData{
		Machine:  machine,
		Watchers: watchersResp.Watchers,
	})
}

func (h *Handler) handleMachineCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.render(w, "machines.html", machineListData{Error: "invalid form data"})
		return
	}

	name := r.FormValue("name")
	ip := r.FormValue("ip")
	sshPortStr := r.FormValue("ssh_port")
	sshUser := r.FormValue("ssh_user")
	sshKey := r.FormValue("ssh_key")

	if name == "" || ip == "" || sshUser == "" || sshKey == "" {
		resp, _ := h.machineSvc.GetMachines(r.Context(), &pb.EmptyRequest{})
		var machines []*pb.MachineResponse
		if resp != nil {
			machines = resp.Machines
		}
		h.render(w, "machines.html", machineListData{
			Machines: machines,
			Error:    "name, ip, ssh_user and ssh_key are required",
		})
		return
	}

	sshPort := int32(22)
	if sshPortStr != "" {
		if p, err := strconv.Atoi(sshPortStr); err == nil {
			sshPort = int32(p)
		}
	}

	_, err := h.machineSvc.CreateMachine(r.Context(), &pb.InitializeMachineRequest{
		Name:          name,
		Ip:            ip,
		SshPort:       sshPort,
		SshUser:       sshUser,
		SshPrivateKey: sshKey,
	})
	if err != nil {
		resp, _ := h.machineSvc.GetMachines(r.Context(), &pb.EmptyRequest{})
		var machines []*pb.MachineResponse
		if resp != nil {
			machines = resp.Machines
		}
		h.render(w, "machines.html", machineListData{
			Machines: machines,
			Error:    fmt.Sprintf("create machine: %v", err),
		})
		return
	}

	http.Redirect(w, r, "/machines", http.StatusSeeOther)
}

func (h *Handler) handleMachineDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	_, err := h.machineSvc.DeleteMachine(r.Context(), &pb.DeleteMachineRequest{Name: name})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// htmx: redirect to the machine list after deletion.
	w.Header().Set("HX-Redirect", "/machines")
	w.WriteHeader(http.StatusOK)
}
