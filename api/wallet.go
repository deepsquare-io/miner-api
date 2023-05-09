package api

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type Wallet struct {
	Wallet string `json:"walletId"`
}

func SaveWallet(c *gin.Context) {

	// get wallet id from http request
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

	os.Setenv("WALLET_ID", wallet.Wallet)
	c.JSON(http.StatusOK, gin.H{"walletId": os.Getenv("WALLET_ID")})

}
