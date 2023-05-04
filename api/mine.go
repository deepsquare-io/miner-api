package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"text/template"

	"github.com/gin-gonic/gin"
	"github.com/squarefactory/miner-api/autoswitch"
)

func MineStart(c *gin.Context) {

	// get algo and pool
	algo := Algo{}
	bestAlgo := autoswitch.GetBestAlgo(c)
	algo.Algo = bestAlgo
	algo.Pool = bestAlgo + ".auto.nicehash.com:443"

	// get wallet id
	data, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error})
		return
	}

	wallet := Wallet{}
	if err := json.Unmarshal(data, &wallet); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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
