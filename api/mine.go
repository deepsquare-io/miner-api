package api

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math"
	"mime/multipart"
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
	"zelhash":     "125_4",
	"zhash":       "144_5",
}

const (
	GPUJobName = "gpu-auto-mining"
	CPUJobName = "cpu-auto-mining"
	user       = "root"
	APIUri     = "https://miner.internal"
)

var (
	lastWalletID string
	lastUsage    float64
)

func MineStart(w http.ResponseWriter, r *http.Request, s *autoswitch.Switcher) {
	slurm := scheduler.NewSlurm(&executor.Shell{}, user)

	// Convert usage slider value to percentage
	usage, err := strconv.ParseFloat(r.FormValue("usage"), 64)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error()})
		log.Printf("failed to parse usage value: %s", err)
		return
	}
	lastUsage = usage
	percent := usage / 100

	// Compute maxGPU
	maxGPU, err := slurm.FindMaxGPU(r.Context())
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err.Error()})
		log.Printf("failed to compute maxGPU: %s", err)
		return
	}

	// Compute GPU replica numbers
	GPUReplicas := int(math.Floor((percent) * float64(maxGPU)))
	// make sure GPU replicas > 0
	if GPUReplicas <= 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, Error{Error: "usage not defined"})
		log.Printf("usage not defined: %s", err)
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
	CPUPerTasks := int(math.Floor((percent) * float64((maxCPU-maxGPU)/maxNode)))
	if CPUPerTasks <= 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, Error{Error: "usage not defined"})
		log.Printf("usage not defined: %s", err)
		return
	}

	walletID := r.FormValue("walletId")
	if len(walletID) == 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, Error{Error: "wallet not defined"})
		log.Printf("wallet not defined: %s", err)
		return
	}
	lastWalletID = walletID

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
		Algo:     AlgoGminer[bestAlgo],
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
		Algo:   "rx/0",
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

func RestartMiners(ctx context.Context) error {

	slurm := scheduler.NewSlurm(&executor.Shell{}, user)

	if _, err := slurm.FindRunningJobByName(ctx, &scheduler.FindRunningJobByNameRequest{
		Name: GPUJobName,
		User: user,
	}); err != nil {
		log.Printf("no jobs are currently running: %s", err)
		return err
	}

	resp, err := http.Post(APIUri+"/stop", "", nil)
	if err != nil {
		log.Printf("failed to send /stop request : %s", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		log.Printf("successfully stopped jobs")
	} else {
		log.Printf("api responded to /stop with %d status code", resp.StatusCode)
		return fmt.Errorf("api responded to /stop with %d status code", resp.StatusCode)
	}
	// wait for the jobs to finish properly
	time.Sleep(time.Duration(10) * time.Second)

	// Create a new multipart buffer
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add the form fields
	if err := writer.WriteField("walletId", lastWalletID); err != nil {
		log.Printf("failed to write walletID to form: %s", err)
		return err
	}
	if err := writer.WriteField("usage", fmt.Sprintf("%f", lastUsage)); err != nil {
		log.Printf("failed to write usage to form: %s", err)
		return err
	}

	// Close the multipart writer to finalize the form data
	writer.Close()

	req, err := http.NewRequest("POST", APIUri+"/start", body)
	if err != nil {
		log.Printf("failed to create new start request: %s", err)
		return err
	}

	// Set the content type header
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Perform the request
	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		log.Printf("failed to send /start request: %s", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		log.Printf("successfully restarted jobs")
		return nil
	} else {
		log.Printf("api responded to /start with %d status code", resp.StatusCode)
		return fmt.Errorf("api responded to /start with %d status code", resp.StatusCode)
	}
}
