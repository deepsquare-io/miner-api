package scheduler

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/squarefactory/miner-api/utils"
)

const QosName = "mining"

type Slurm struct {
	executor  Executor
	adminUser string
}

func NewSlurm(
	executor Executor,
	adminUser string,
) *Slurm {
	return &Slurm{
		executor:  executor,
		adminUser: adminUser,
	}
}

// CancelJob kills a job using scancel command.
func (s *Slurm) CancelJob(ctx context.Context, req *CancelRequest) error {
	cmd := fmt.Sprintf("scancel --name=%s --me", req.Name)
	_, err := s.executor.ExecAs(ctx, req.User, cmd)
	if err != nil {
		log.Printf("cancel failed: %s", err)
	}
	return err
}

// Submit a sbatch definition script to the SLURM controller using the sbatch command.
func (s *Slurm) Submit(ctx context.Context, req *SubmitRequest) (string, error) {
	eof := utils.GenerateRandomString(10)

	cmd := fmt.Sprintf(`sbatch \
  --job-name=%s \
  --qos=%s \
  --output=/tmp/miner-%d.log \
  --parsable << '%s'
%s
%s`,
		req.Name,
		QosName,
		time.Now().UnixMilli(),
		eof,
		req.Body,
		eof,
	)
	out, err := s.executor.ExecAs(ctx, req.User, cmd)
	if err != nil {
		log.Printf("submit failed: %s", err)
		return strings.TrimSpace(strings.TrimRight(string(out), "\n")), err
	}

	return strings.TrimSpace(strings.TrimRight(string(out), "\n")), nil
}

// HealthCheck runs squeue to check if the queue is running
func (s *Slurm) HealthCheck(ctx context.Context) error {
	_, err := s.executor.ExecAs(ctx, s.adminUser, "squeue")
	if err != nil {
		log.Printf("healthcheck failed: %s", err)
	}
	return err
}

// FindRunningJobByName find a running job using squeue.
func (s *Slurm) FindRunningJobByName(
	ctx context.Context,
	req *FindRunningJobByNameRequest,
) (int, error) {
	cmd := fmt.Sprintf("squeue --name %s -O JobId:256 --noheader", req.Name)
	out, err := s.executor.ExecAs(ctx, req.User, cmd)
	if err != nil {
		log.Printf("FindRunningJobByName failed: %s", err)
		return 0, err
	}

	return strconv.Atoi(strings.TrimSpace(strings.TrimRight(out, "\n")))
}

func (s *Slurm) FindMaxGpu(ctx context.Context) (int, error) {
	cmd := "scontrol show nodes | grep CfgTRES | sed -E 's|.*gres/gpu=([^,]*)|\\1|g'"
	out, err := s.executor.ExecAs(ctx, s.adminUser, cmd)
	if err != nil {
		log.Printf("FindMaxGpu failed: %s", err)
		return 0, err
	}

	out = strings.TrimSpace(string(out))
	lines := strings.Split(out, "\n")

	maxGpu := 0
	for _, line := range lines {
		num, err := strconv.Atoi(line)
		if err != nil {
			log.Printf("Failed to convert %q to integer: %s", line, err)
			return 0, err
		}
		maxGpu += num
	}

	return maxGpu, nil
}
