package converter

// // EnvironmentToProto converts the domain model of the environment to a protobuf message
// func EnvironmentToProto(environment *models.Environment) (*environmentv1.Environment, error) {
// 	if environment == nil {
// 		return nil, nil
// 	}

// 	// Convert the JSON config to a protobuf Struct
// 	var configMap map[string]interface{}
// 	if err := json.Unmarshal(environment.Config, &configMap); err != nil {
// 		return nil, err
// 	}

// 	configStruct, err := structpb.NewStruct(configMap)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &environmentv1.Environment{
// 		Id:        environment.ID.String(),
// 		Name:      environment.Name,
// 		Config:    configStruct,
// 		TenantId:  environment.TenantID.String(),
// 		CreatedAt: timestamppb.New(environment.CreatedAt),
// 		UpdatedAt: timestamppb.New(environment.UpdatedAt),
// 	}, nil
// }

// // EnvironmentsToProto converts a list of domain models of environments to protobuf messages
// func EnvironmentsToProto(environments []*models.Environment) ([]*environmentv1.Environment, error) {
// 	result := make([]*environmentv1.Environment, len(environments))
// 	for i, environment := range environments {
// 		protoEnv, err := EnvironmentToProto(environment)
// 		if err != nil {
// 			return nil, err
// 		}
// 		result[i] = protoEnv
// 	}
// 	return result, nil
// }

// // CreateEnvironmentRequestFromProto converts a protobuf request to a domain model
// func CreateEnvironmentRequestFromProto(req *environmentv1.CreateEnvironmentRequest) (*models.CreateEnvironmentRequest, error) {
// 	if req == nil {
// 		return nil, nil
// 	}

// 	tenantID, err := uuid.Parse(req.TenantId)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Convert the protobuf Struct to JSON
// 	configBytes, err := json.Marshal(req.Config.AsMap())
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &models.CreateEnvironmentRequest{
// 		Name:     req.Name,
// 		Config:   configBytes,
// 		TenantID: tenantID,
// 	}, nil
// }

// // UpdateEnvironmentRequestFromProto converts a protobuf request to a domain model
// func UpdateEnvironmentRequestFromProto(req *environmentv1.UpdateEnvironmentRequest) (*models.UpdateEnvironmentRequest, error) {
// 	if req == nil {
// 		return nil, nil
// 	}

// 	// Convert the protobuf Struct to JSON
// 	configBytes, err := json.Marshal(req.Config.AsMap())
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &models.UpdateEnvironmentRequest{
// 		Name:   req.Name,
// 		Config: configBytes,
// 	}, nil
// }
