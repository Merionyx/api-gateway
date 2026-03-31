package storage

import (
	"sync"

	authv1 "merionyx/api-gateway/pkg/api/auth/v1"
)

// AccessStorage хранит права доступа в памяти
type AccessStorage struct {
	contracts map[string]*authv1.ContractAccess // key: contract_name
	version   int64
	mu        sync.RWMutex
}

func NewAccessStorage() *AccessStorage {
	return &AccessStorage{
		contracts: make(map[string]*authv1.ContractAccess),
	}
}

// SetAccessConfig устанавливает полную конфигурацию
func (s *AccessStorage) SetAccessConfig(config *authv1.AccessConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.contracts = make(map[string]*authv1.ContractAccess)
	for _, contract := range config.Contracts {
		s.contracts[contract.ContractName] = contract
	}
	s.version = config.Version
}

// ApplyUpdate применяет инкрементальное обновление
func (s *AccessStorage) ApplyUpdate(update *authv1.AccessUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Добавляем новые контракты
	for _, contract := range update.AddedContracts {
		s.contracts[contract.ContractName] = contract
	}

	// Обновляем существующие
	for _, contract := range update.UpdatedContracts {
		s.contracts[contract.ContractName] = contract
	}

	// Удаляем
	for _, contractName := range update.RemovedContracts {
		delete(s.contracts, contractName)
	}

	s.version = update.Version
}

// CheckAccess проверяет, имеет ли приложение доступ к контракту
func (s *AccessStorage) CheckAccess(contractName, appID, environment string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	contract, ok := s.contracts[contractName]
	if !ok {
		return false
	}

	// Если контракт не secure - разрешаем всем
	if !contract.Secure {
		return true
	}

	// Проверяем, есть ли приложение в списке
	for _, app := range contract.Apps {
		if app.AppId == appID {
			return true
		}
	}

	return false
}

// GetVersion возвращает текущую версию конфигурации
func (s *AccessStorage) GetVersion() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

// GetContractsCount возвращает количество контрактов
func (s *AccessStorage) GetContractsCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.contracts)
}
