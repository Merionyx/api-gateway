package apiclient

import (
	"strings"

	apiserverclient "github.com/merionyx/api-gateway/internal/cli/apiserver/client"
)

// ExportRequest is the JSON body for POST /api/v1/contracts/export (OpenAPI ContractsExportRequest).
type ExportRequest = apiserverclient.ContractsExportRequest

// ExportFile is one contract file in a successful export response.
type ExportFile = apiserverclient.ContractsExportFile

// NewExportRequest maps CLI flag strings to an export body. Empty optional path/contract are omitted in JSON.
func NewExportRequest(repository, ref, path, contract string) ExportRequest {
	r := ExportRequest{
		Repository: strings.TrimSpace(repository),
		Ref:        strings.TrimSpace(ref),
	}
	if p := strings.TrimSpace(path); p != "" {
		r.Path = &p
	}
	if c := strings.TrimSpace(contract); c != "" {
		r.ContractName = &c
	}
	return r
}

// ExportRequestPath returns the optional repository path or "".
func ExportRequestPath(r ExportRequest) string {
	if r.Path == nil {
		return ""
	}
	return *r.Path
}

// ExportRequestContractName returns the optional single-contract filter or "".
func ExportRequestContractName(r ExportRequest) string {
	if r.ContractName == nil {
		return ""
	}
	return *r.ContractName
}
