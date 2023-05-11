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
	jobName = "auto-mining"
	user    = "root"
)

func MineStart(w http.ResponseWriter, r *http.Request, s *autoswitch.Switcher) {
	slurm := scheduler.NewSlurm(&executor.Shell{}, user)

	percent, err := strconv.ParseFloat(r.FormValue("usage"), 64)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error()})
		log.Printf("failed to parse usage value: %s", err)
		return
	}

	maxGpu, err := slurm.FindMaxGpu(r.Context())
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error()})
		log.Printf("failed to compute maxGpu: %s", err)
		return
	}

	// compute replicas
	replicas := int(math.Floor((percent / 100) * float64(maxGpu)))
	// make sure replicas > 0
	if replicas <= 0 {
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
		Name: jobName,
		User: user,
	}); err == nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, Error{Error: fmt.Sprintf("job %d is already running", jobID)})
		return
	}

	// get best algo and corresponding pool
	bestAlgo, err := s.GetBestAlgo(r.Context())
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error()})
		log.Printf("GetBestAlgo failed: %s", err)
		return
	}

	tmpl := template.Must(template.New("jobTemplate").Parse(JobTemplate))
	var jobScript bytes.Buffer
	if err := tmpl.Execute(&jobScript, struct {
		Wallet   string
		Algo     string
		Pool     string
		Replicas int
	}{
		Wallet:   walletID,
		Algo:     bestAlgo,
		Pool:     bestAlgo + ".auto.nicehash.com:443",
		Replicas: replicas,
	}); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error()})
		log.Printf("templating failed: %s", err)
		return
	}

	out, err := slurm.Submit(r.Context(), &scheduler.SubmitRequest{
		Name: jobName,
		User: user,
		Body: jobScript.String(),
	})
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error(), Data: out})
		log.Printf("submit failed: %s", err)
		return
	}

	render.JSON(w, r, OK{fmt.Sprintf("Mining job %s started", out)})
}

func MineStop(w http.ResponseWriter, r *http.Request) {
	slurm := scheduler.NewSlurm(&executor.Shell{}, user)
	err := slurm.CancelJob(r.Context(), &scheduler.CancelRequest{
		Name: jobName,
		User: user,
	})

	if err != nil {
		render.JSON(w, r, Error{
			Error: err.Error(),
			Data:  "Mining job stopped",
		})
		log.Printf("mine stop failed: %s", err)
		return
	}
	render.JSON(w, r, OK{"Mining job stopped"})
}
