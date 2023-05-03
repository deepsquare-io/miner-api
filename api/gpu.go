package api

import (
	"bytes"
	"log"
	"os/exec"
	"strconv"
)

func GetGPU() int {
	cmd := exec.Command("sh", "-c", "scontrol show nodes | grep CfgTRES | sed -E \"s|.*gres/gpu=([^,]*)|\\1|g\"")

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		log.Fatalf("Failed to run command: %v", err)
	}

	output, err := strconv.Atoi(stdout.String())
	if err != nil {
		log.Fatalf("Failed to count available gpus")
	}

	return output
}
