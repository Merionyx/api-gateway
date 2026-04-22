package registry

import (
	"context"
	"sort"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/pagination"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"
)

// ControllerReadUseCase serves HTTP registry reads for controllers.
type ControllerReadUseCase struct {
	controllers interfaces.ControllerRepository
}

func NewControllerReadUseCase(controllers interfaces.ControllerRepository) *ControllerReadUseCase {
	return &ControllerReadUseCase{controllers: controllers}
}

func (u *ControllerReadUseCase) ListControllers(ctx context.Context, limit *int, cursor *string) ([]models.ControllerInfo, *string, bool, error) {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseRegistryPkg, "ListControllers"))
	defer span.End()
	all, err := u.controllers.ListControllers(ctx)
	if err != nil {
		telemetry.MarkError(span, err)
		return nil, nil, false, err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ControllerID < all[j].ControllerID })
	lim := pagination.ResolveLimit(limit)
	return pagination.PageSlice(all, lim, cursor)
}

func (u *ControllerReadUseCase) ListControllersByTenant(ctx context.Context, tenant string, limit *int, cursor *string) ([]models.ControllerInfo, *string, bool, error) {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseRegistryPkg, "ListControllersByTenant"))
	defer span.End()
	all, err := u.controllers.ListControllers(ctx)
	if err != nil {
		telemetry.MarkError(span, err)
		return nil, nil, false, err
	}
	filtered := make([]models.ControllerInfo, 0, len(all))
	for i := range all {
		if all[i].Tenant == tenant {
			filtered = append(filtered, all[i])
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].ControllerID < filtered[j].ControllerID })
	lim := pagination.ResolveLimit(limit)
	return pagination.PageSlice(filtered, lim, cursor)
}

func (u *ControllerReadUseCase) GetController(ctx context.Context, controllerID string) (*models.ControllerInfo, error) {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseRegistryPkg, "GetController"))
	defer span.End()
	info, err := u.controllers.GetController(ctx, controllerID)
	if err != nil {
		telemetry.MarkError(span, err)
	}
	return info, err
}

func (u *ControllerReadUseCase) GetHeartbeat(ctx context.Context, controllerID string) (time.Time, error) {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseRegistryPkg, "GetHeartbeat"))
	defer span.End()
	t, err := u.controllers.GetHeartbeat(ctx, controllerID)
	if err != nil {
		telemetry.MarkError(span, err)
	}
	return t, err
}
