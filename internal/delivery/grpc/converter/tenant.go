package converter

// // TenantToProto converts the domain model of the tenant to a protobuf message
// func TenantToProto(tenant *models.Tenant) *tenantv1.Tenant {
// 	if tenant == nil {
// 		return nil
// 	}

// 	return &tenantv1.Tenant{
// 		Id:        tenant.ID.String(),
// 		Name:      tenant.Name,
// 		CreatedAt: timestamppb.New(tenant.CreatedAt),
// 		UpdatedAt: timestamppb.New(tenant.UpdatedAt),
// 	}
// }

// // TenantsToProto converts a list of domain models of tenants to protobuf messages
// func TenantsToProto(tenants []*models.Tenant) []*tenantv1.Tenant {
// 	result := make([]*tenantv1.Tenant, len(tenants))
// 	for i, tenant := range tenants {
// 		result[i] = TenantToProto(tenant)
// 	}
// 	return result
// }

// // CreateTenantRequestFromProto converts a protobuf request to a domain model
// func CreateTenantRequestFromProto(req *tenantv1.CreateTenantRequest) *models.CreateTenantRequest {
// 	if req == nil {
// 		return nil
// 	}

// 	return &models.CreateTenantRequest{
// 		Name: req.Name,
// 	}
// }

// // UpdateTenantRequestFromProto converts a protobuf request to a domain model
// func UpdateTenantRequestFromProto(req *tenantv1.UpdateTenantRequest) *models.UpdateTenantRequest {
// 	if req == nil {
// 		return nil
// 	}

// 	return &models.UpdateTenantRequest{
// 		Name: req.Name,
// 	}
// }
