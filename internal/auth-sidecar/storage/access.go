package storage

import (
	"log"
	"strings"
	"sync"

	authv1 "merionyx/api-gateway/pkg/api/auth/v1"
)

// AccessStorage stores access rights in memory
type AccessStorage struct {
	contracts    map[string]*authv1.ContractAccess // key: contract_name
	prefixToName map[string]string                 // key: prefix, value: contract_name
	version      int64
	mu           sync.RWMutex
}

func NewAccessStorage() *AccessStorage {
	return &AccessStorage{
		contracts:    make(map[string]*authv1.ContractAccess),
		prefixToName: make(map[string]string),
	}
}

// ReceiveAccessConfig receives the full configuration
func (s *AccessStorage) ReceiveAccessConfig(contractName string) *authv1.ContractAccess {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("ReceiveAccessConfig: %s", contractName)
	log.Printf("Contracts: %v", s.contracts)

	contract, ok := s.contracts[contractName]
	if !ok {
		return nil
	}

	return contract
}

// FindContractByPath finds a contract by its path
func (s *AccessStorage) FindContractByPath(path string) *authv1.ContractAccess {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var longestPrefix string
	var matchedContract *authv1.ContractAccess

	// Find the longest prefix that matches the start of the path
	for prefix, contractName := range s.prefixToName {
		if strings.HasPrefix(path, prefix) {
			if len(prefix) > len(longestPrefix) {
				longestPrefix = prefix
				matchedContract = s.contracts[contractName]
			}
		}
	}

	return matchedContract
}

// SetAccessConfig sets the full configuration
func (s *AccessStorage) SetAccessConfig(config *authv1.AccessConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, contract := range config.Contracts {
		s.contracts[contract.ContractName] = contract
		if contract.Prefix != "" {
			s.prefixToName[contract.Prefix] = contract.ContractName
		}
	}
	s.version = config.Version
}

// ApplyUpdate applies an incremental update
func (s *AccessStorage) ApplyUpdate(update *authv1.AccessUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add new contracts
	for _, contract := range update.AddedContracts {
		s.contracts[contract.ContractName] = contract
		if contract.Prefix != "" {
			s.prefixToName[contract.Prefix] = contract.ContractName
		}
	}

	// Update existing contracts
	for _, contract := range update.UpdatedContracts {
		s.contracts[contract.ContractName] = contract
		if contract.Prefix != "" {
			s.prefixToName[contract.Prefix] = contract.ContractName
		}
	}

	// Remove contracts
	for _, contractName := range update.RemovedContracts {
		contract := s.contracts[contractName]
		if contract != nil && contract.Prefix != "" {
			delete(s.prefixToName, contract.Prefix)
		}
		delete(s.contracts, contractName)
	}

	s.version = update.Version
}

// CheckAccess checks if the application has access to the contract
func (s *AccessStorage) CheckAccess(contractName, appID, environment string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	contract, ok := s.contracts[contractName]
	if !ok {
		return false
	}

	// If the contract is not secure - allow all
	if !contract.Secure {
		return true
	}

	// Check if the application is in the list
	for _, app := range contract.Apps {
		if app.AppId == appID {
			return true
		}
	}

	return false
}

// GetVersion returns the current configuration version
func (s *AccessStorage) GetVersion() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

// GetContractsCount returns the number of contracts
func (s *AccessStorage) GetContractsCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.contracts)
}
