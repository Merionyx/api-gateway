package builder

import "merionyx/api-gateway/control-plane/internal/domain/interfaces"

type xdsBuilder struct {
	inMemoryServiceRepository interfaces.InMemoryServiceRepository
}

func NewXDSBuilder(inMemoryServiceRepository interfaces.InMemoryServiceRepository) interfaces.XDSBuilder {
	return &xdsBuilder{
		inMemoryServiceRepository: inMemoryServiceRepository,
	}
}
