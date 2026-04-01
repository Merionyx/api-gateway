package models

// ContractSnapshot is the contract routing/access payload stored in etcd and used for xDS.
type ContractSnapshot struct {
	Name                  string           `json:"name"`
	Prefix                string           `json:"prefix"`
	Upstream              ContractUpstream `json:"upstream"`
	AllowUndefinedMethods bool             `json:"allow_undefined_methods"`
	Access                Access           `json:"access"`
}

type ContractUpstream struct {
	Name string `json:"name"`
}

type Access struct {
	Secure bool  `json:"secure"`
	Apps   []App `json:"apps"`
}

type App struct {
	AppID        string   `json:"app_id"`
	Environments []string `json:"environments,omitempty"`
}
