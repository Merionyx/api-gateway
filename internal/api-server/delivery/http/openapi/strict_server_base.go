package openapi

import (
	"github.com/merionyx/api-gateway/internal/api-server/container"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
)

// StrictOpenAPIServer implements generated StrictServerInterface and calls usecase/domain logic directly.
type StrictOpenAPIServer struct {
	c *container.Container
}

func NewStrictOpenAPIServer(c *container.Container) apiserver.StrictServerInterface {
	if c == nil {
		panic("strict openapi server requires container")
	}
	if c.PermissionEvaluator == nil {
		panic("strict openapi server requires permission evaluator")
	}
	return &StrictOpenAPIServer{c: c}
}

var _ apiserver.StrictServerInterface = (*StrictOpenAPIServer)(nil)
