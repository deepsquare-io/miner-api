package api

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"text/template"

	"github.com/go-chi/render"
	"github.com/squarefactory/miner-api/autoswitch"
	"github.com/squarefactory/miner-api/executor"
	"github.com/squarefactory/miner-api/scheduler"
)

const (
	GPUJobName = "gpu-auto-mining"
	CPUJobName = "cpu-auto-mining"
	user       = "root"
)

func MineStart(w http.ResponseWriter, r *http.Request, s *autoswitch.Switcher) {
	slurm := scheduler.NewSlurm(&executor.Shell{}, user)

	// Convert usage slider value to percentage
	percent, err := strconv.ParseFloat(r.FormValue("usage"), 64)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error()})
		log.Printf("failed to parse usage value: %s", err)
		return
	}

	// Compute maxGPU
	maxGPU, err := slurm.FindMaxGPU(r.Context())
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error()})
		log.Printf("failed to compute maxGPU: %s", err)
		return
	}

	// Compute GPU replica numbers
	GPUReplicas := int(math.Floor((percent / 100) * float64(maxGPU)))
	// make sure GPU replicas > 0
	if GPUReplicas <= 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, Error{Error: "usage not defined"})
		return
	}

	// Compute maxNode
	maxNode, err := slurm.FindMaxNode(r.Context())
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error()})
		log.Printf("failed to compute maxNode: %s", err)
		return
	}

	maxCPU, err := slurm.FindMaxCPU(r.Context())
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error()})
		log.Printf("failed to compute maxCPU: %s", err)
		return
	}

	// Compute number of cores used by each miner, keeping 1 core per GPU miner
	CPUPerTasks := int(math.Floor((percent / 100) * float64((maxCPU-maxGPU)/maxNode)))
	if CPUPerTasks <= 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, Error{Error: "usage not defined"})
		return
	}

	walletID := r.FormValue("walletId")
	if len(walletID) == 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, Error{Error: "wallet not defined"})
		return
	}

	// Check if already running
	if jobID, err := slurm.FindRunningJobByName(r.Context(), &scheduler.FindRunningJobByNameRequest{
		Name: GPUJobName,
		User: user,
	}); err == nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, Error{Error: fmt.Sprintf("job %d is already running", jobID)})
		return
	}

	// get best algo and corresponding pool for gpu mining job
	bestAlgo, err := s.GetBestAlgo(r.Context())
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error()})
		log.Printf("GetBestAlgo failed: %s", err)
		return
	}

	// Templating gpu mining job
	GPUtmpl := template.Must(template.New("jobTemplate").Parse(GPUTemplate))
	var GPUJobScript bytes.Buffer
	if err := GPUtmpl.Execute(&GPUJobScript, struct {
		Wallet   string
		Algo     string
		Pool     string
		Replicas int
	}{
		Wallet:   walletID,
		Algo:     bestAlgo,
		Pool:     bestAlgo + ".auto.nicehash.com:443",
		Replicas: GPUReplicas,
	}); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error()})
		log.Printf("templating failed: %s", err)
		return
	}

	// submitting gpu mining job
	GPUout, err := slurm.Submit(r.Context(), &scheduler.SubmitRequest{
		Name: GPUJobName,
		User: user,
		Body: GPUJobScript.String(),
	})
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error(), Data: GPUout})
		log.Printf("submit failed: %s", err)
		return
	}

	// Check if already running
	if jobID, err := slurm.FindRunningJobByName(r.Context(), &scheduler.FindRunningJobByNameRequest{
		Name: CPUJobName,
		User: user,
	}); err == nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, Error{Error: fmt.Sprintf("job %d is already running", jobID)})
		return
	}

	// Templating cpu mining job
	CPUTmpl := template.Must(template.New("CPUTemplate").Parse(CPUTemplate))
	var CPUJobScript bytes.Buffer
	if err := CPUTmpl.Execute(&CPUJobScript, struct {
		Wallet string
		Algo   string
		Pool   string
		Node   int
		Core   int
	}{
		Wallet: walletID,
		Algo:   "randomx",
		Pool:   "randomxmonero.auto.nicehash.com:443",
		Node:   maxNode,
		Core:   CPUPerTasks,
	}); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error()})
		log.Printf("templating failed: %s", err)
		return
	}

	// submitting cpu mining job
	CPUout, err := slurm.Submit(r.Context(), &scheduler.SubmitRequest{
		Name: CPUJobName,
		User: user,
		Body: CPUJobScript.String(),
	})
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error(), Data: CPUout})
		log.Printf("submit failed: %s", err)
		return
	}

	render.JSON(w, r, OK{fmt.Sprintf("Mining jobs %s, %s started", GPUout, CPUout)})

}

func MineStop(w http.ResponseWriter, r *http.Request) {
	slurm := scheduler.NewSlurm(&executor.Shell{}, user)
	// cancelling GPU job
	err := slurm.CancelJob(r.Context(), &scheduler.CancelRequest{
		Name: GPUJobName,
		User: user,
	})

	if err != nil {
		render.JSON(w, r, Error{
			Error: err.Error(),
			Data:  "GPU mining job stopped",
		})
		log.Printf("GPU mine stop failed: %s", err)
		return
	}

	// cancelling CPU job
	err = slurm.CancelJob(r.Context(), &scheduler.CancelRequest{
		Name: CPUJobName,
		User: user,
	})

	if err != nil {
		render.JSON(w, r, Error{
			Error: err.Error(),
			Data:  "CPU mining job stopped",
		})
		log.Printf("CPU mine stop failed: %s", err)
		return
	}

	render.JSON(w, r, OK{"Mining job stopped"})
}
