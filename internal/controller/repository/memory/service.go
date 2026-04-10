package memory

import (
	"fmt"
	"sync"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

type ServiceRepository struct {
	mu sync.RWMutex

	static      map[string]models.StaticServiceConfig
	kubeGlobals []models.StaticServiceConfig
}

func NewServiceRepository() interfaces.InMemoryServiceRepository {
	return &ServiceRepository{
		static: make(map[string]models.StaticServiceConfig),
	}
}

func (r *ServiceRepository) Initialize(config *config.Config) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, service := range config.Services.Static {
		r.static[service.Name] = models.StaticServiceConfig{
			Name:     service.Name,
			Upstream: service.Upstream,
		}
	}
	return nil
}

func (r *ServiceRepository) SetKubernetesGlobalServices(services []models.StaticServiceConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kubeGlobals = append([]models.StaticServiceConfig(nil), services...)
}

func (r *ServiceRepository) GetService(name string) (*models.StaticServiceConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if service, ok := r.static[name]; ok {
		return &service, nil
	}
	for i := range r.kubeGlobals {
		if r.kubeGlobals[i].Name == name {
			s := r.kubeGlobals[i]
			return &s, nil
		}
	}
	return nil, fmt.Errorf("service %s not found", name)
}

func (r *ServiceRepository) ListServices() ([]models.StaticServiceConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	services := make([]models.StaticServiceConfig, 0, len(r.static)+len(r.kubeGlobals))
	for _, service := range r.static {
		services = append(services, service)
	}
	services = append(services, r.kubeGlobals...)
	return services, nil
}
