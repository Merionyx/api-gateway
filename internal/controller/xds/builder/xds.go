package builder

import "merionyx/api-gateway/internal/controller/domain/interfaces"

type xdsBuilder struct {
	inMemoryServiceRepository interfaces.InMemoryServiceRepository
}

func NewXDSBuilder(inMemoryServiceRepository interfaces.InMemoryServiceRepository) interfaces.XDSBuilder {
	return &xdsBuilder{
		inMemoryServiceRepository: inMemoryServiceRepository,
	}
}
