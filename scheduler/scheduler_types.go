package scheduler

import "context"

type Executor interface {
	ExecAs(ctx context.Context, user string, cmd string) (string, error)
}

type CancelRequest struct {
	// Name of the job
	Name string
	// User is a UNIX User used for impersonation.
	User string
}

type SubmitRequest struct {
	// Name of the job
	Name string
	// User is a UNIX User used for impersonation.
	User string
	// Body of the job
	Body string
}

type FindRunningJobByNameRequest struct {
	// Name of the job
	Name string
	// User is a UNIX User used for impersonation. This user should be SLURM admin.
	User string
}
