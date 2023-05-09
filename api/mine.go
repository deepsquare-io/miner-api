package api

import (
	"bytes"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"text/template"

	"github.com/gin-gonic/gin"
	"github.com/squarefactory/miner-api/autoswitch"
)

func MineStart(c *gin.Context) {

	// get wallet id from env
	wallet := Wallet{}
	walletID := os.Getenv("WALLET_ID")
	if len(walletID) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "wallet not defined"})
		return
	}
	wallet.Wallet = walletID

	// get best algo and corresponding pool
	algo := Algo{}
	bestAlgo := autoswitch.GetBestAlgo(c)
	algo.Algo = bestAlgo
	// generating stratum
	algo.Pool = bestAlgo + ".auto.nicehash.com:443"

	tmpl := template.Must(template.New("jobTemplate").Parse(JobTemplate))
	var jobScript bytes.Buffer
	if err := tmpl.Execute(&jobScript, wallet); err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error()})
		return
	}
	if err := tmpl.Execute(&jobScript, algo); err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error()})
		return
	}

	jobScriptFile, err := os.Create("mining_job.sh")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer jobScriptFile.Close()
	jobScriptFile.WriteString(jobScript.String())

	cmd := exec.Command("sh", "-c", "sbatch mining_job.sh")
	if err := cmd.Run(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "Mining job started"})
		output, _ := cmd.Output()

		jobIDRegex := regexp.MustCompile(`\d+`)
		jobID := string(jobIDRegex.Find(output))

		os.Setenv("MINING_JOB_ID", jobID)
	}
}

func MineStop(c *gin.Context) {

	cmd := exec.Command("sh", "-c", "scancel", os.Getenv("MINING_JOB_ID"))
	if err := cmd.Run(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "Mining job stopped"})
	}
}
