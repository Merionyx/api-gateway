package converter

import (
	"encoding/json"

	"merionyx/api-gateway/control-plane/internal/domain/models"
	listenerv1 "merionyx/api-gateway/control-plane/pkg/api/listener/v1"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ListenerToProto converts the domain model of the listener to a protobuf message
func ListenerToProto(listener *models.Listener) (*listenerv1.Listener, error) {
	if listener == nil {
		return nil, nil
	}

	// Convert the JSON config to a protobuf Struct
	var configMap map[string]interface{}
	if err := json.Unmarshal(listener.Config, &configMap); err != nil {
		return nil, err
	}

	configStruct, err := structpb.NewStruct(configMap)
	if err != nil {
		return nil, err
	}

	return &listenerv1.Listener{
		Id:            listener.ID.String(),
		Name:          listener.Name,
		Config:        configStruct,
		EnvironmentId: listener.EnvironmentID.String(),
		CreatedAt:     timestamppb.New(listener.CreatedAt),
		UpdatedAt:     timestamppb.New(listener.UpdatedAt),
	}, nil
}

// ListenersToProto converts a list of domain models of listeners to protobuf messages
func ListenersToProto(listeners []*models.Listener) ([]*listenerv1.Listener, error) {
	result := make([]*listenerv1.Listener, len(listeners))
	for i, listener := range listeners {
		protoListener, err := ListenerToProto(listener)
		if err != nil {
			return nil, err
		}
		result[i] = protoListener
	}
	return result, nil
}

// CreateListenerRequestFromProto converts a protobuf request to a domain model
func CreateListenerRequestFromProto(req *listenerv1.CreateListenerRequest) (*models.CreateListenerRequest, error) {
	if req == nil {
		return nil, nil
	}

	environmentID, err := uuid.Parse(req.EnvironmentId)
	if err != nil {
		return nil, err
	}

	// Convert the protobuf Struct to JSON
	configBytes, err := json.Marshal(req.Config.AsMap())
	if err != nil {
		return nil, err
	}

	return &models.CreateListenerRequest{
		Name:          req.Name,
		Config:        configBytes,
		EnvironmentID: environmentID,
	}, nil
}

// UpdateListenerRequestFromProto converts a protobuf request to a domain model
func UpdateListenerRequestFromProto(req *listenerv1.UpdateListenerRequest) (*models.UpdateListenerRequest, error) {
	if req == nil {
		return nil, nil
	}

	// Convert the protobuf Struct to JSON
	configBytes, err := json.Marshal(req.Config.AsMap())
	if err != nil {
		return nil, err
	}

	return &models.UpdateListenerRequest{
		Name:   req.Name,
		Config: configBytes,
	}, nil
}
