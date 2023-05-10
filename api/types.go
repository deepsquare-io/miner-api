package api

type Error struct {
	Error error  `json:"error"`
	Data  string `json:"data,omitempty"`
}

type OK struct {
	Data string `json:"data"`
}
