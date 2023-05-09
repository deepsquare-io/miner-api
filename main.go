package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/squarefactory/miner-api/api"
)

func main() {

	r := gin.Default()

	r.POST("/start", api.MineStart)
	r.POST("/stop", api.MineStop)
	r.POST("/save-wallet", api.SaveWallet)

	r.GET("/health", api.Health)
	r.StaticFile("/", "./web/index.html")

	listenAddress := os.Getenv("LISTEN_ADDRESS")
	if len(listenAddress) == 0 {
		listenAddress = ":8080"
	}

	err := r.Run(listenAddress)
	if err != nil {
		log.Fatal(err)
	}
}
