package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Resource(resource string) schema.GroupResource {
	return GroupVersion.WithResource(resource).GroupResource()
}

func init() {
	SchemeBuilder.Register(
		&Environment{}, &EnvironmentList{},
		&ContractBundle{}, &ContractBundleList{},
		&ContractRepository{}, &ContractRepositoryList{},
		&GatewayUpstream{}, &GatewayUpstreamList{},
	)
}
