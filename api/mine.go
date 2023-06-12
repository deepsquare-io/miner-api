package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"text/template"
	"time"

	"github.com/go-chi/render"
	"github.com/squarefactory/miner-api/autoswitch"
	"github.com/squarefactory/miner-api/executor"
	"github.com/squarefactory/miner-api/scheduler"
)

var AlgoGminer = map[string]string{
	"autolykos":   "autolykos",
	"beamv3":      "beamhash",
	"cuckoocycle": "cuckoocycle",
	"cuckatoo32":  "cuckatoo32",
	"etchash":     "etchash",
	"ethash":      "ethash",
	"kawpow":      "kawpow",
	"kheavyhash":  "kheavyhash",
	"octopus":     "octopus",
	"zelhash":     "equihash125_4",
	"zhash":       "equihash144_5",
}

const (
	GPUJobName = "gpu-auto-mining"
	CPUJobName = "cpu-auto-mining"
	user       = "root"
)

var (
	lastWalletID string
	lastUsage    float64
	jobState     = false // Indicates if a job is supposed to be running or not. (True = Running)
)

type Replicas struct {
	maxGPU      int
	replicasGPU int
	maxNode     int
	maxCPU      int
	replicasCPU int
}

type JobData struct {
	walletID string
	algo     string
}

func MineStart(w http.ResponseWriter, r *http.Request, s *autoswitch.Switcher) {
	slurm := scheduler.NewSlurm(&executor.Shell{}, user)

	// Check if GPU job already running
	if jobID, err := slurm.FindRunningJobByName(r.Context(), &scheduler.FindRunningJobByNameRequest{
		Name: GPUJobName,
		User: user,
	}); err == nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, Error{Error: fmt.Sprintf("job %d is already running", jobID)})
		return
	}

	// Check if CPU job already running
	if jobID, err := slurm.FindRunningJobByName(r.Context(), &scheduler.FindRunningJobByNameRequest{
		Name: CPUJobName,
		User: user,
	}); err == nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, Error{Error: fmt.Sprintf("job %d is already running", jobID)})
		return
	}

	walletID := r.FormValue("walletId")
	if len(walletID) == 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, Error{Error: "wallet not defined"})
		log.Printf("wallet not defined")
		return
	}
	lastWalletID = walletID

	// Convert usage slider value to percentage
	usage, err := strconv.ParseFloat(r.FormValue("usage"), 64)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error()})
		log.Printf("failed to parse usage value: %s", err)
		return
	}
	lastUsage = usage

	// Compute Replicas
	replicas, err := ComputeReplicas(slurm, r.Context(), usage/100)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error()})
		log.Printf("failed to compute replicas: %s", err)
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

	data := JobData{
		algo:     bestAlgo,
		walletID: walletID,
	}

	out, err := StartJobs(slurm, r.Context(), replicas, data)
	if err != nil {
		log.Printf("failed to start jobs: %s", err)
		return
	}

	render.JSON(w, r, OK{fmt.Sprintf("Mining jobs %s started", out)})
	jobState = true

}

func MineStop(w http.ResponseWriter, r *http.Request) {
	slurm := scheduler.NewSlurm(&executor.Shell{}, user)
	// cancelling GPU job
	if err := StopJobs(slurm, r.Context()); err != nil {
		log.Printf("failed to stop jobs: %s", err)
		return
	}

	render.JSON(w, r, OK{"Mining job stopped"})
	jobState = false
}

func RestartMiners(ctx context.Context, s *autoswitch.Switcher) error {
	slurm := scheduler.NewSlurm(&executor.Shell{}, user)

	if !jobState {
		log.Printf("no jobs are currently running")
		return errors.New("jobs are not running, unable to restart")
	}

	// Stop miners
	if err := StopJobs(slurm, ctx); err != nil {
		log.Printf("failed to stop jobs")
		return err
	}

	// Wait for jobs to stop completely
	time.Sleep(time.Duration(10) * time.Second)

	// Compute Replicas
	replicas, err := ComputeReplicas(slurm, ctx, lastUsage/100)
	if err != nil {
		log.Printf("failed to compute replicas")
		return err
	}

	// Get best algo
	bestAlgo, err := s.GetBestAlgo(ctx)
	if err != nil {
		log.Printf("failed to get best algo")
		return err
	}

	data := JobData{
		walletID: lastWalletID,
		algo:     bestAlgo,
	}

	// Restart miners
	if _, err := StartJobs(slurm, ctx, replicas, data); err != nil {
		log.Printf("failed to restart jobs")
		return err
	}

	return nil
}

func ComputeReplicas(slurm *scheduler.Slurm, ctx context.Context, percent float64) (Replicas, error) {
	// Compute maxGPU
	maxGPU, err := slurm.FindMaxGPU(ctx)
	if err != nil {
		log.Printf("failed to compute maxGPU: %s", err)
		return Replicas{}, err
	}

	// Compute GPU replica numbers
	GPUReplicas := int(math.Floor((percent) * float64(maxGPU)))
	// make sure GPU replicas > 0
	if GPUReplicas <= 0 {
		log.Printf("usage not defined: %s", err)
		return Replicas{}, err
	}

	// Compute maxNode
	maxNode, err := slurm.FindMaxNode(ctx)
	if err != nil {
		log.Printf("failed to compute maxNode: %s", err)
		return Replicas{}, err
	}

	maxCPU, err := slurm.FindMaxCPU(ctx)
	if err != nil {
		log.Printf("failed to compute maxCPU: %s", err)
		return Replicas{}, err
	}

	// Compute number of cores used by each miner, keeping 1 core per GPU miner
	CPUPerTasks := int(math.Floor((percent) * float64((maxCPU-maxGPU)/maxNode)))
	if CPUPerTasks <= 0 {
		log.Printf("usage not defined: %s", err)
		return Replicas{}, err
	}

	return Replicas{
		maxGPU:      maxGPU,
		maxNode:     maxNode,
		maxCPU:      maxCPU,
		replicasGPU: GPUReplicas,
		replicasCPU: CPUPerTasks,
	}, nil
}

func StopJobs(slurm *scheduler.Slurm, ctx context.Context) error {
	// cancelling GPU job
	err := slurm.CancelJob(ctx, &scheduler.CancelRequest{
		Name: GPUJobName,
		User: user,
	})

	if err != nil {
		log.Printf("GPU mine stop failed: %s", err)
		return err
	}

	// cancelling CPU job
	err = slurm.CancelJob(ctx, &scheduler.CancelRequest{
		Name: CPUJobName,
		User: user,
	})

	if err != nil {
		log.Printf("CPU mine stop failed: %s", err)
		return err
	}

	log.Printf("successfully stopped jobs")
	return nil
}

func StartJobs(slurm *scheduler.Slurm, ctx context.Context, replicas Replicas, data JobData) (string, error) {

	// Templating gpu mining job
	GPUtmpl := template.Must(template.New("jobTemplate").Parse(GPUTemplate))
	var GPUJobScript bytes.Buffer
	if err := GPUtmpl.Execute(&GPUJobScript, struct {
		Wallet   string
		Algo     string
		Pool     string
		Replicas int
	}{
		Wallet:   data.walletID,
		Algo:     AlgoGminer[data.algo],
		Pool:     data.algo + ".auto.nicehash.com:443",
		Replicas: replicas.replicasCPU,
	}); err != nil {
		log.Printf("templating failed: %s", err)
		return "", err
	}

	// submitting gpu mining job
	GPUout, err := slurm.Submit(ctx, &scheduler.SubmitRequest{
		Name: GPUJobName,
		User: user,
		Body: GPUJobScript.String(),
	})
	if err != nil {
		log.Printf("submit failed: %s", err)
		return "", err
	}
	log.Printf("successfully restarted gpu job: %s", GPUout)

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
		Wallet: lastWalletID,
		Algo:   "rx/0",
		Pool:   "randomxmonero.auto.nicehash.com:443",
		Node:   replicas.maxNode,
		Core:   replicas.replicasCPU,
	}); err != nil {
		log.Printf("templating failed: %s", err)
		return "", err
	}

	// submitting cpu mining job
	CPUout, err := slurm.Submit(ctx, &scheduler.SubmitRequest{
		Name: CPUJobName,
		User: user,
		Body: CPUJobScript.String(),
	})
	if err != nil {
		log.Printf("submit failed: %s", err)
		return "", err
	}
	log.Printf("successfully restarted cpu job: %s", CPUout)

	log.Printf("successfully restarted jobs")
	return fmt.Sprintf("%s"+"%s", GPUout, CPUout), nil
}
