package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/squarefactory/miner-api/executor"
	"github.com/squarefactory/miner-api/scheduler"

	"github.com/go-chi/render"
)

func Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	slurm := scheduler.NewSlurm(&executor.Shell{}, user)

	if err := slurm.HealthCheck(ctx); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, Error{Error: err})
		log.Printf("health failed: %s", err)
		return
	}
	render.JSON(w, r, OK{"ok"})
}
