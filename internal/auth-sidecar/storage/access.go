package storage

import (
	"log/slog"
	"sort"
	"strings"
	"sync"

	authv1 "merionyx/api-gateway/pkg/api/auth/v1"
)

// AccessStorage stores access rights in memory
type AccessStorage struct {
	contracts      map[string]*authv1.ContractAccess // key: contract_name
	prefixToName   map[string]string                 // key: prefix, value: contract_name
	sortedPrefixes []string                          // longest prefix first for FindContractByPath
	version        int64
	mu             sync.RWMutex
}

func NewAccessStorage() *AccessStorage {
	return &AccessStorage{
		contracts:      make(map[string]*authv1.ContractAccess),
		prefixToName:   make(map[string]string),
		sortedPrefixes: nil,
	}
}

func (s *AccessStorage) rebuildPrefixOrderLocked() {
	if len(s.prefixToName) == 0 {
		s.sortedPrefixes = nil
		return
	}
	prefs := make([]string, 0, len(s.prefixToName))
	for p := range s.prefixToName {
		prefs = append(prefs, p)
	}
	sort.Slice(prefs, func(i, j int) bool {
		if len(prefs[i]) != len(prefs[j]) {
			return len(prefs[i]) > len(prefs[j])
		}
		return prefs[i] < prefs[j]
	})
	s.sortedPrefixes = prefs
}

// ReceiveAccessConfig receives the full configuration
func (s *AccessStorage) ReceiveAccessConfig(contractName string) *authv1.ContractAccess {
	s.mu.Lock()
	defer s.mu.Unlock()

	slog.Debug("access storage lookup", "contract", contractName, "contracts_count", len(s.contracts))

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

	for _, prefix := range s.sortedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return s.contracts[s.prefixToName[prefix]]
		}
	}

	return nil
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
	s.rebuildPrefixOrderLocked()
	s.version = config.Version
}

// ApplyUpdate applies an incremental update
func (s *AccessStorage) ApplyUpdate(update *authv1.AccessUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, contract := range update.AddedContracts {
		s.contracts[contract.ContractName] = contract
		if contract.Prefix != "" {
			s.prefixToName[contract.Prefix] = contract.ContractName
		}
	}

	for _, contract := range update.UpdatedContracts {
		s.contracts[contract.ContractName] = contract
		if contract.Prefix != "" {
			s.prefixToName[contract.Prefix] = contract.ContractName
		}
	}

	for _, contractName := range update.RemovedContracts {
		contract := s.contracts[contractName]
		if contract != nil && contract.Prefix != "" {
			delete(s.prefixToName, contract.Prefix)
		}
		delete(s.contracts, contractName)
	}

	s.rebuildPrefixOrderLocked()
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

	if !contract.Secure {
		return true
	}

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
