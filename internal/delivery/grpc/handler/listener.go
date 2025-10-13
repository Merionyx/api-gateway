package handler

import (
	"context"

	"merionyx/api-gateway/control-plane/internal/delivery/grpc/converter"
	"merionyx/api-gateway/control-plane/internal/domain/interfaces"
	listenerv1 "merionyx/api-gateway/control-plane/pkg/api/listener/v1"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListenerHandler gRPC handler for listeners
type ListenerHandler struct {
	listenerv1.UnimplementedListenerServiceServer
	listenerUseCase interfaces.ListenerUseCase
}

// NewListenerHandler creates a new instance of ListenerHandler
func NewListenerHandler(listenerUseCase interfaces.ListenerUseCase) *ListenerHandler {
	return &ListenerHandler{
		listenerUseCase: listenerUseCase,
	}
}

// CreateListener creates a new listener
func (h *ListenerHandler) CreateListener(ctx context.Context, req *listenerv1.CreateListenerRequest) (*listenerv1.CreateListenerResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be empty")
	}

	domainReq, err := converter.CreateListenerRequestFromProto(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	listener, err := h.listenerUseCase.CreateListener(ctx, domainReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoListener, err := converter.ListenerToProto(listener)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &listenerv1.CreateListenerResponse{
		Listener: protoListener,
	}, nil
}

// GetListener gets a listener by ID
func (h *ListenerHandler) GetListener(ctx context.Context, req *listenerv1.GetListenerRequest) (*listenerv1.GetListenerResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "listener ID is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid listener ID")
	}

	listener, err := h.listenerUseCase.GetListenerByID(ctx, id)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	protoListener, err := converter.ListenerToProto(listener)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &listenerv1.GetListenerResponse{
		Listener: protoListener,
	}, nil
}

// GetListenerByName gets a listener by name
func (h *ListenerHandler) GetListenerByName(ctx context.Context, req *listenerv1.GetListenerByNameRequest) (*listenerv1.GetListenerByNameResponse, error) {
	if req == nil || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "listener name is required")
	}

	listener, err := h.listenerUseCase.GetListenerByName(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	protoListener, err := converter.ListenerToProto(listener)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &listenerv1.GetListenerByNameResponse{
		Listener: protoListener,
	}, nil
}

// GetListeners gets a list of all listeners
func (h *ListenerHandler) GetListeners(ctx context.Context, req *listenerv1.GetListenersRequest) (*listenerv1.GetListenersResponse, error) {
	listeners, err := h.listenerUseCase.GetAllListeners(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoListeners, err := converter.ListenersToProto(listeners)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &listenerv1.GetListenersResponse{
		Listeners: protoListeners,
	}, nil
}

// GetListenersByEnvironment gets listeners by environment
func (h *ListenerHandler) GetListenersByEnvironment(ctx context.Context, req *listenerv1.GetListenersByEnvironmentRequest) (*listenerv1.GetListenersByEnvironmentResponse, error) {
	if req == nil || req.EnvironmentId == "" {
		return nil, status.Error(codes.InvalidArgument, "environment ID is required")
	}

	environmentID, err := uuid.Parse(req.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid environment ID")
	}

	listeners, err := h.listenerUseCase.GetListenersByEnvironmentID(ctx, environmentID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoListeners, err := converter.ListenersToProto(listeners)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &listenerv1.GetListenersByEnvironmentResponse{
		Listeners: protoListeners,
	}, nil
}

// UpdateListener updates a listener
func (h *ListenerHandler) UpdateListener(ctx context.Context, req *listenerv1.UpdateListenerRequest) (*listenerv1.UpdateListenerResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "listener ID is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid listener ID")
	}

	domainReq, err := converter.UpdateListenerRequestFromProto(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	listener, err := h.listenerUseCase.UpdateListener(ctx, id, domainReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoListener, err := converter.ListenerToProto(listener)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &listenerv1.UpdateListenerResponse{
		Listener: protoListener,
	}, nil
}

// DeleteListener deletes a listener
func (h *ListenerHandler) DeleteListener(ctx context.Context, req *listenerv1.DeleteListenerRequest) (*listenerv1.DeleteListenerResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "listener ID is required")
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid listener ID")
	}

	if err := h.listenerUseCase.DeleteListener(ctx, id); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &listenerv1.DeleteListenerResponse{}, nil
}

// MapListenerToEnvironment maps a listener to an environment
func (h *ListenerHandler) MapListenerToEnvironment(ctx context.Context, req *listenerv1.MapListenerToEnvironmentRequest) (*listenerv1.MapListenerToEnvironmentResponse, error) {
	if req == nil || req.ListenerId == "" || req.EnvironmentId == "" {
		return nil, status.Error(codes.InvalidArgument, "listener ID and environment ID are required")
	}

	listenerID, err := uuid.Parse(req.ListenerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid listener ID")
	}

	environmentID, err := uuid.Parse(req.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid environment ID")
	}

	if err := h.listenerUseCase.MapListenerToEnvironment(ctx, listenerID, environmentID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &listenerv1.MapListenerToEnvironmentResponse{}, nil
}

// UnmapListenerFromEnvironment unmaps a listener from an environment
func (h *ListenerHandler) UnmapListenerFromEnvironment(ctx context.Context, req *listenerv1.UnmapListenerFromEnvironmentRequest) (*listenerv1.UnmapListenerFromEnvironmentResponse, error) {
	if req == nil || req.ListenerId == "" || req.EnvironmentId == "" {
		return nil, status.Error(codes.InvalidArgument, "listener ID and environment ID are required")
	}

	listenerID, err := uuid.Parse(req.ListenerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid listener ID")
	}

	environmentID, err := uuid.Parse(req.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid environment ID")
	}

	if err := h.listenerUseCase.UnmapListenerFromEnvironment(ctx, listenerID, environmentID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &listenerv1.UnmapListenerFromEnvironmentResponse{}, nil
}
