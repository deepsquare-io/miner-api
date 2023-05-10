package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/render"
)

var walletID string

type Wallet struct {
	Wallet string `json:"walletId"`
}

func SaveWallet(w http.ResponseWriter, r *http.Request) {
	// get wallet id from http request
	wallet := Wallet{}
	if err := json.NewDecoder(r.Body).Decode(&wallet); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, Error{Error: err.Error()})
		return
	}

	walletID = wallet.Wallet
	render.JSON(w, r, OK{walletID})
}
