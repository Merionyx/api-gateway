package memory

import (
	"fmt"
	"merionyx/api-gateway/internal/controller/config"
	"merionyx/api-gateway/internal/controller/domain/interfaces"
	"merionyx/api-gateway/internal/controller/domain/models"
)

type ServiceRepository struct {
	services map[string]models.StaticServiceConfig
}

func NewServiceRepository() interfaces.InMemoryServiceRepository {
	return &ServiceRepository{
		services: make(map[string]models.StaticServiceConfig),
	}
}

func (r *ServiceRepository) Initialize(config *config.Config) error {
	for _, service := range config.Services.Static {
		r.services[service.Name] = models.StaticServiceConfig{
			Name:     service.Name,
			Upstream: service.Upstream,
		}
	}
	return nil
}

func (r *ServiceRepository) GetService(name string) (*models.StaticServiceConfig, error) {
	service, ok := r.services[name]
	if !ok {
		return nil, fmt.Errorf("service %s not found", name)
	}
	return &service, nil
}

func (r *ServiceRepository) ListServices() ([]models.StaticServiceConfig, error) {
	services := make([]models.StaticServiceConfig, 0, len(r.services))
	for _, service := range r.services {
		services = append(services, service)
	}
	return services, nil
}
