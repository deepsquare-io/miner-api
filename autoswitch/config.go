package autoswitch

type Gpu struct {
	X69XT   int `yaml:"amd69xt"`
	X68XT   int `yaml:"amd68xt"`
	X67XT   int `yaml:"amd67xt"`
	X66XT   int `yaml:"amd66xt"`
	VII     int `yaml:"vii"`
	X5700XT int `yaml:"amd5700xt"`
	X5700   int `yaml:"amd5700"`
	X5600XT int `yaml:"amd5600xt"`
	Vega64  int `yaml:"vega64"`
	Vega56  int `yaml:"vega56"`
	X4090   int `yaml:"nvi4090"`
	X4080   int `yaml:"nvi4080"`
	X47Ti   int `yaml:"nvi47Ti"`
	X47     int `yaml:"nvi47"`
	X39Ti   int `yaml:"nvi39Ti"`
	X3090   int `yaml:"nvi3090"`
	X38Ti   int `yaml:"nvi38Ti"`
	X3080   int `yaml:"nvi3080"`
	X37Ti   int `yaml:"nvi37Ti"`
	X3070   int `yaml:"nvi3070"`
}

type Algorithm struct {
	HashRate int `yaml:"hash-rate"`
	Power    int `yaml:"power"`
}

type General struct {
	PollingFrequency int     `yaml:"polling_frequency"`
	PowerCostPerKwh  float64 `yaml:"power_cost_per_kwh"`
	Threshold        float64 `yaml:"threshold"`
}
