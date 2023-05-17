package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	_ "embed"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/squarefactory/miner-api/api"
	"github.com/squarefactory/miner-api/autoswitch"
	"gopkg.in/yaml.v3"
)

//go:embed web/index.html
var f string

func main() {

	configPath := os.Getenv("CONFIG_PATH")
	cb, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatal(err)
	}
	var config autoswitch.Config
	if err := yaml.Unmarshal(cb, &config); err != nil {
		log.Fatal(err)
	}

	switcher := &autoswitch.Switcher{
		Config: &config,
	}
	r := chi.NewRouter()

	r.Use(middleware.Logger)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		render.HTML(w, r, f)
	})
	r.Post("/start", func(w http.ResponseWriter, r *http.Request) {

		api.MineStart(w, r, switcher)
	})
	r.Post("/stop", api.MineStop)
	r.Get("/health", api.Health)

	listenAddress := os.Getenv("LISTEN_ADDRESS")
	if len(listenAddress) == 0 {
		listenAddress = ":8080"
	}
	l, err := net.Listen("tcp", listenAddress)
	if err != nil {
		log.Fatal(err)
	}
	if err := http.Serve(l, r); err != nil {
		log.Fatal(err)
	}

	// context for the relaunch job goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		ticker := time.NewTicker(time.Duration(time.Duration(switcher.Config.General.PollingFrequency).Minutes()))
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				err := api.RestartMiners(ctx)
				if err != nil {
					fmt.Println("Failed to relaunch jobs:", err)
				}
			}
		}
	}()
}
