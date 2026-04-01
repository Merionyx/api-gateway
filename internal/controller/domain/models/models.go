package models

type Environment struct {
	Name      string
	Type      string
	Snapshots []ContractSnapshot
	Services  *EnvironmentServiceConfig
	Bundles   *EnvironmentBundleConfig
}

type EnvironmentServiceConfig struct {
	Static []StaticServiceConfig
}

type StaticServiceConfig struct {
	Name     string
	Upstream string
}

type EnvironmentBundleConfig struct {
	Static []StaticContractBundleConfig
}

type StaticContractBundleConfig struct {
	Name       string
	Repository string
	Ref        string
	Path       string
}

// Snapshots UseCase models
type UpdateSnapshotRequest struct {
	Environment string
}

type UpdateSnapshotResponse struct {
	Success             bool
	UpdatedEnvironments []string
}

type GetSnapshotStatusRequest struct {
	Environment string
}

type GetSnapshotStatusResponse struct {
	Environment    string
	Version        string
	ContractsCount int32
	ClustersCount  int32
	RoutesCount    int32
}

// Environments UseCase models
type CreateEnvironmentRequest struct {
	Name     string
	Type     string
	Bundles  *EnvironmentBundleConfig
	Services *EnvironmentServiceConfig
}

type UpdateEnvironmentRequest struct {
	Name     string
	Bundles  *EnvironmentBundleConfig
	Services *EnvironmentServiceConfig
}

// Schemas UseCase models
type SyncContractBundleRequest struct {
	Repository string
	Ref        string
	Bundle     string
	Path       string
	Force      bool
}

type SyncContractBundleResponse struct {
	Snapshots []ContractSnapshot
	FromCache bool
}

type SyncAllContractsRequest struct {
	Environment string
}

type SyncAllContractsResponse struct {
	SyncedCount int32
	Errors      []string
}
