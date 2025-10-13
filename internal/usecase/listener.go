package usecase

import (
	"context"
	"fmt"
	"time"

	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/domain/models"

	"github.com/google/uuid"
)

type listenerUseCase struct {
	listenerRepo    interfaces.ListenerRepository
	environmentRepo interfaces.EnvironmentRepository
}

// NewListenerUseCase creates a new instance of ListenerUseCase
func NewListenerUseCase(
	listenerRepo interfaces.ListenerRepository,
	environmentRepo interfaces.EnvironmentRepository,
) interfaces.ListenerUseCase {
	return &listenerUseCase{
		listenerRepo:    listenerRepo,
		environmentRepo: environmentRepo,
	}
}

func (uc *listenerUseCase) CreateListener(ctx context.Context, req *models.CreateListenerRequest) (*models.Listener, error) {
	// Check if the environment exists
	_, err := uc.environmentRepo.GetByID(ctx, req.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("environment not found: %w", err)
	}

	// Check if the listener with the same name does not exist
	existingListener, err := uc.listenerRepo.GetByName(ctx, req.Name)
	if err == nil && existingListener != nil {
		return nil, fmt.Errorf("listener with name '%s' already exists", req.Name)
	}

	listener := &models.Listener{
		ID:            uuid.New(),
		Name:          req.Name,
		Config:        req.Config,
		EnvironmentID: req.EnvironmentID,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := uc.listenerRepo.Create(ctx, listener); err != nil {
		return nil, fmt.Errorf("error creating listener: %w", err)
	}

	// Map the listener to the environment
	if err := uc.listenerRepo.MapToEnvironment(ctx, listener.ID, req.EnvironmentID); err != nil {
		return nil, fmt.Errorf("error mapping listener to environment: %w", err)
	}

	return listener, nil
}

func (uc *listenerUseCase) GetListenerByID(ctx context.Context, id uuid.UUID) (*models.Listener, error) {
	listener, err := uc.listenerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("error getting listener by ID: %w", err)
	}

	return listener, nil
}

func (uc *listenerUseCase) GetListenerByName(ctx context.Context, name string) (*models.Listener, error) {
	listener, err := uc.listenerRepo.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("error getting listener by name: %w", err)
	}

	return listener, nil
}

func (uc *listenerUseCase) GetAllListeners(ctx context.Context) ([]*models.Listener, error) {
	listeners, err := uc.listenerRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting list of listeners: %w", err)
	}

	return listeners, nil
}

func (uc *listenerUseCase) GetListenersByEnvironmentID(ctx context.Context, environmentID uuid.UUID) ([]*models.Listener, error) {
	// Check if the environment exists
	_, err := uc.environmentRepo.GetByID(ctx, environmentID)
	if err != nil {
		return nil, fmt.Errorf("environment not found: %w", err)
	}

	listeners, err := uc.listenerRepo.GetByEnvironmentID(ctx, environmentID)
	if err != nil {
		return nil, fmt.Errorf("error getting listeners by environment ID: %w", err)
	}

	return listeners, nil
}

func (uc *listenerUseCase) UpdateListener(ctx context.Context, id uuid.UUID, req *models.UpdateListenerRequest) (*models.Listener, error) {
	// Check if the listener exists
	listener, err := uc.listenerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("listener not found: %w", err)
	}

	// Check if the listener with the new name does not exist (if the name changed)
	if listener.Name != req.Name {
		existingListener, err := uc.listenerRepo.GetByName(ctx, req.Name)
		if err == nil && existingListener != nil {
			return nil, fmt.Errorf("listener with name '%s' already exists", req.Name)
		}
	}

	listener.Name = req.Name
	listener.Config = req.Config
	listener.UpdatedAt = time.Now()

	if err := uc.listenerRepo.Update(ctx, listener); err != nil {
		return nil, fmt.Errorf("error updating listener: %w", err)
	}

	return listener, nil
}

func (uc *listenerUseCase) DeleteListener(ctx context.Context, id uuid.UUID) error {
	// Check if the listener exists
	_, err := uc.listenerRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("listener not found: %w", err)
	}

	if err := uc.listenerRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("error deleting listener: %w", err)
	}

	return nil
}

func (uc *listenerUseCase) MapListenerToEnvironment(ctx context.Context, listenerID, environmentID uuid.UUID) error {
	// Check if the listener and environment exist
	_, err := uc.listenerRepo.GetByID(ctx, listenerID)
	if err != nil {
		return fmt.Errorf("listener not found: %w", err)
	}

	_, err = uc.environmentRepo.GetByID(ctx, environmentID)
	if err != nil {
		return fmt.Errorf("environment not found: %w", err)
	}

	if err := uc.listenerRepo.MapToEnvironment(ctx, listenerID, environmentID); err != nil {
		return fmt.Errorf("error mapping listener to environment: %w", err)
	}

	return nil
}

func (uc *listenerUseCase) UnmapListenerFromEnvironment(ctx context.Context, listenerID, environmentID uuid.UUID) error {
	// Check if the listener and environment exist
	_, err := uc.listenerRepo.GetByID(ctx, listenerID)
	if err != nil {
		return fmt.Errorf("listener not found: %w", err)
	}

	_, err = uc.environmentRepo.GetByID(ctx, environmentID)
	if err != nil {
		return fmt.Errorf("environment not found: %w", err)
	}

	if err := uc.listenerRepo.UnmapFromEnvironment(ctx, listenerID, environmentID); err != nil {
		return fmt.Errorf("error unmapping listener from environment: %w", err)
	}

	return nil
}
