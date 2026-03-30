package builder

import "merionyx/api-gateway/control-plane/internal/repository/memory"

type XDSBuilder struct {
	inMemoryServiceRepository *memory.ServiceRepository
}

func NewXDSBuilder(inMemoryServiceRepository *memory.ServiceRepository) *XDSBuilder {
	return &XDSBuilder{
		inMemoryServiceRepository: inMemoryServiceRepository,
	}
}
