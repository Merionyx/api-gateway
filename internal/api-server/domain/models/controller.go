package models

type ControllerInfo struct {
	ControllerID string
	Tenant       string
	Environments []EnvironmentInfo
}

type EnvironmentInfo struct {
	Name    string
	Bundles []BundleInfo
}

type BundleInfo struct {
	Name       string
	Repository string
	Ref        string
	Path       string
}
