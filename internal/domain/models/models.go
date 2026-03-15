package models

import "merionyx/api-gateway/control-plane/internal/repository/git"

type Environment struct {
	Name      string
	Snapshots []git.ContractSnapshot
	Services  *EnvironmentServiceConfig
	Contracts *EnvironmentContractConfig
}

type EnvironmentServiceConfig struct {
	Static []StaticServiceConfig
}

type StaticServiceConfig struct {
	Name     string
	Upstream string
}
type EnvironmentContractConfig struct {
	Static []StaticContractConfig
}

type StaticContractConfig struct {
	Name       string
	Repository string
	Ref        string
}
