package api

import _ "embed"

//go:embed templates/job.tmpl
var GPUTemplate string

//go:embed templates/cpu.tmpl
var CPUTemplate string
