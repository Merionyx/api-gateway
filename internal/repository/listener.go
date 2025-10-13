package repository

import (
	"context"
	"database/sql"
	"fmt"

	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	"merionyx/api-gateway/control-plane/internal/domain/models"
	pg_queries "merionyx/api-gateway/control-plane/internal/queries"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type listenerRepository struct {
	db      *pgxpool.Pool
	queries *pg_queries.Queries
}

// NewListenerRepository creates a new instance of ListenerRepository
func NewListenerRepository(db *pgxpool.Pool) interfaces.ListenerRepository {
	return &listenerRepository{
		db:      db,
		queries: pg_queries.New(db),
	}
}

func (r *listenerRepository) Create(ctx context.Context, listener *models.Listener) error {
	err := r.queries.CreateListener(ctx, pg_queries.CreateListenerParams{
		Uuid:   listener.ID,
		Name:   listener.Name,
		Config: listener.Config,
	})
	if err != nil {
		return fmt.Errorf("error creating listener in DB: %w", err)
	}
	return nil
}

func (r *listenerRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Listener, error) {
	dbListener, err := r.queries.GetListenerByUUID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("listener with ID %s not found", id)
		}
		return nil, fmt.Errorf("error getting listener from DB: %w", err)
	}

	return &models.Listener{
		ID:     dbListener.Uuid,
		Name:   dbListener.Name,
		Config: dbListener.Config,
	}, nil
}

func (r *listenerRepository) GetByName(ctx context.Context, name string) (*models.Listener, error) {
	dbListener, err := r.queries.GetListenerByName(ctx, name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("listener with name %s not found", name)
		}
		return nil, fmt.Errorf("error getting listener from DB: %w", err)
	}

	return &models.Listener{
		ID:     dbListener.Uuid,
		Name:   dbListener.Name,
		Config: dbListener.Config,
	}, nil
}

func (r *listenerRepository) GetAll(ctx context.Context) ([]*models.Listener, error) {
	dbListeners, err := r.queries.GetListeners(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting list of listeners from DB: %w", err)
	}

	listeners := make([]*models.Listener, len(dbListeners))
	for i, dbListener := range dbListeners {
		listeners[i] = &models.Listener{
			ID:     dbListener.Uuid,
			Name:   dbListener.Name,
			Config: dbListener.Config,
		}
	}

	return listeners, nil
}

func (r *listenerRepository) GetByEnvironmentID(ctx context.Context, environmentID uuid.UUID) ([]*models.Listener, error) {
	dbListeners, err := r.queries.GetListenersByEnvironmentUUID(ctx, environmentID)
	if err != nil {
		return nil, fmt.Errorf("error getting listeners by environment ID from DB: %w", err)
	}

	listeners := make([]*models.Listener, len(dbListeners))
	for i, dbListener := range dbListeners {
		listeners[i] = &models.Listener{
			ID:            dbListener.Uuid,
			Name:          dbListener.Name,
			Config:        dbListener.Config,
			EnvironmentID: environmentID,
		}
	}

	return listeners, nil
}

func (r *listenerRepository) Update(ctx context.Context, listener *models.Listener) error {
	err := r.queries.UpdateListener(ctx, pg_queries.UpdateListenerParams{
		Uuid:   listener.ID,
		Name:   listener.Name,
		Config: listener.Config,
	})
	if err != nil {
		return fmt.Errorf("error updating listener in DB: %w", err)
	}
	return nil
}

func (r *listenerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// First delete the relationships with environments
	err := r.queries.DeleteListenerEnvironmentMappings(ctx, id)
	if err != nil {
		return fmt.Errorf("error deleting listener environment relationships: %w", err)
	}

	// Then delete the listener itself
	err = r.queries.DeleteListener(ctx, id)
	if err != nil {
		return fmt.Errorf("error deleting listener from DB: %w", err)
	}
	return nil
}

func (r *listenerRepository) MapToEnvironment(ctx context.Context, listenerID, environmentID uuid.UUID) error {
	err := r.queries.MapListenerToEnvironment(ctx, pg_queries.MapListenerToEnvironmentParams{
		ListenerUuid:    listenerID,
		EnvironmentUuid: environmentID,
	})
	if err != nil {
		return fmt.Errorf("error mapping listener to environment in DB: %w", err)
	}
	return nil
}

func (r *listenerRepository) UnmapFromEnvironment(ctx context.Context, listenerID, environmentID uuid.UUID) error {
	err := r.queries.UnmapListenerFromEnvironment(ctx, pg_queries.UnmapListenerFromEnvironmentParams{
		ListenerUuid:    listenerID,
		EnvironmentUuid: environmentID,
	})
	if err != nil {
		return fmt.Errorf("error unmapping listener from environment in DB: %w", err)
	}
	return nil
}
