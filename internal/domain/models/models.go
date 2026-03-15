package models

import "merionyx/api-gateway/control-plane/internal/repository/git"

type Environment struct {
	Name      string
	Snapshots []git.ContractSnapshot
	Services  *EnvironmentServiceConfig
	Contracts *EnvironmentContractConfig
}

type EnvironmentServiceConfig struct {
	Type string
	List []EnvironmentService
}

type EnvironmentService struct {
	Name     string
	Upstream string
}

type EnvironmentContractConfig struct {
	Type string
	List []EnvironmentContract
}

type EnvironmentContract struct {
	Name       string
	Repository string
	Ref        string
}
