package api

const JobTemplate = `
#!/bin/env bash
#SBATCH --ntasks=1
#SBATCH --gpus-per-task=1
#SBATCH --cpus-per-task=1
#SBATCH --mem-per-cpu=16G
#SBATCH --qos=mining

cont='registry-1.deepsquare.run#library/gminer'

srun --ntasks=1 --gpus-per-task=1 --cpus-per-task=1 --mem-per-cpu=16G --container-image="$cont" \
  bash -c "miner --algo {{.ALGO}} --server {{.POOL}} --proto stratum --ssl 1 --user {{.Wallet}} --pass x"
`

type Wallet struct {
	Wallet string `json:"wallet"`
}

type Algo struct {
	Algo string
	Pool string
}
