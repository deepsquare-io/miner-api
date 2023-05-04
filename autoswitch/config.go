package autoswitch

type Algorithm struct {
	HashRate int `yaml:"hash-rate"`
	Power    int `yaml:"power"`
}

type General struct {
	PollingFrequency int     `yaml:"polling_frequency"`
	PowerCostPerKwh  float64 `yaml:"power_cost_per_kwh"`
	Threshold        float64 `yaml:"threshold"`
}
