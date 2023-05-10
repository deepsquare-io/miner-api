package api

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
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
	// get wallet id from env
	wallet := Wallet{}
	if len(walletID) == 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, Error{Error: errors.New("wallet not defined")})
		return
	}
	wallet.Wallet = walletID

	// get best algo and corresponding pool
	algo := Algo{}
	bestAlgo, err := s.GetBestAlgo(r.Context())

	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err})
		log.Printf("GetBestAlgo failed: %s", err)
		return
	}

	algo.Algo = bestAlgo
	// generating stratum
	algo.Pool = bestAlgo + ".auto.nicehash.com:443"

	// TODO: replace with user value
	tasks := 1

	tmpl := template.Must(template.New("jobTemplate").Parse(JobTemplate))
	var jobScript bytes.Buffer
	if err := tmpl.Execute(&jobScript, struct {
		Wallet Wallet
		Algo   Algo
		Tasks  int
	}{
		Wallet: wallet,
		Algo:   algo,
		Tasks:  tasks,
	}); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err})
		log.Printf("templating failed: %s", err)
		return
	}

	slurm := scheduler.NewSlurm(&executor.Shell{}, user)

	out, err := slurm.Submit(r.Context(), &scheduler.SubmitRequest{
		Name: jobName,
		User: user,
		Body: jobScript.String(),
	})
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err, Data: out})
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
			Error: err,
			Data:  "Mining job stopped",
		})
		log.Printf("mine stop failed: %s", err)
		return
	}
	render.JSON(w, r, OK{"Mining job stopped"})
}
