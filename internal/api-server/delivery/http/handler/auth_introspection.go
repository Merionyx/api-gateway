package handler

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
	"github.com/merionyx/api-gateway/internal/api-server/auth/roles"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/authz"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"
)

// AuthIntrospectionHandler exposes public auth catalog/introspection endpoints.
type AuthIntrospectionHandler struct {
	jwtUseCase  *auth.JWTUseCase
	roleCatalog *roles.Catalog
}

// NewAuthIntrospectionHandler wires auth introspection endpoints.
func NewAuthIntrospectionHandler(jwtUseCase *auth.JWTUseCase, roleCatalog *roles.Catalog) *AuthIntrospectionHandler {
	if roleCatalog == nil {
		roleCatalog, _ = roles.NewCatalog(nil)
	}
	return &AuthIntrospectionHandler{
		jwtUseCase:  jwtUseCase,
		roleCatalog: roleCatalog,
	}
}

// ListRoles returns role catalog with effective permission descriptors (GET /v1/auth/roles).
func (h *AuthIntrospectionHandler) ListRoles(c fiber.Ctx) error {
	roleRows := h.roleCatalog.ListRolePermissions()
	out := make([]apiserver.RolePermissions, 0, len(roleRows))
	for i := range roleRows {
		out = append(out, apiserver.RolePermissions{
			Role:        roleRows[i].RoleID,
			Permissions: permissionDescriptorsFromIDs(roleRows[i].Permissions),
		})
	}
	return c.JSON(out)
}

// ListPermissions returns documented permissions (GET /v1/auth/permissions).
func (h *AuthIntrospectionHandler) ListPermissions(c fiber.Ctx) error {
	byID := make(map[string]string)
	for _, d := range permissions.ListDescriptors() {
		byID[d.ID] = d.Description
	}
	for _, roleRow := range h.roleCatalog.ListRolePermissions() {
		for _, permissionID := range roleRow.Permissions {
			if _, ok := byID[permissionID]; ok {
				continue
			}
			byID[permissionID] = permissions.Describe(permissionID)
		}
	}

	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	out := make([]apiserver.PermissionDescriptor, 0, len(ids))
	for _, id := range ids {
		out = append(out, apiserver.PermissionDescriptor{
			Id:          id,
			Description: byID[id],
		})
	}
	return c.JSON(out)
}

// InspectTokenPermissions validates API token and returns effective permission descriptors (POST /v1/auth/token-permissions).
func (h *AuthIntrospectionHandler) InspectTokenPermissions(c fiber.Ctx) error {
	if h.jwtUseCase == nil {
		return problem.WriteInternal(c, nil)
	}

	var req apiserver.TokenPermissionsRequest
	if err := c.Bind().Body(&req); err != nil {
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest(problem.CodeInvalidJSONBody, "", problem.DetailInvalidJSONBody))
	}

	rawToken := strings.TrimSpace(req.AccessToken)
	if rawToken == "" {
		return problem.Write(c, http.StatusBadRequest, problem.BadRequest(
			"ACCESS_TOKEN_REQUIRED",
			"",
			"Field access_token is required.",
		))
	}

	claims, err := h.jwtUseCase.ParseAndValidateAPIProfileBearerToken(rawToken)
	if err != nil {
		return problem.Write(c, http.StatusUnauthorized, problem.Unauthorized(
			"INVALID_ACCESS_TOKEN",
			"",
			"Provided access token is invalid or expired.",
		))
	}

	subject := claimString(claims, "sub")
	if subject == "" {
		subject = claimString(claims, "email")
	}

	tokenRoles := uniqueSortedStrings(authz.NormalizeRolesValue(claims["roles"]))
	effective := h.roleCatalog.ResolvePermissions(tokenRoles)
	for _, permissionID := range claimStrings(claims, "permissions") {
		effective[permissionID] = struct{}{}
	}
	for _, permissionID := range claimStrings(claims, "scopes") {
		effective[permissionID] = struct{}{}
	}

	permissionIDs := mapKeysSorted(effective)
	return c.JSON(apiserver.TokenPermissionsResponse{
		Subject:     subject,
		Roles:       tokenRoles,
		Permissions: permissionDescriptorsFromIDs(permissionIDs),
	})
}

func permissionDescriptorsFromIDs(ids []string) []apiserver.PermissionDescriptor {
	if len(ids) == 0 {
		return []apiserver.PermissionDescriptor{}
	}
	unique := uniqueSortedStrings(ids)
	out := make([]apiserver.PermissionDescriptor, 0, len(unique))
	for _, permissionID := range unique {
		out = append(out, apiserver.PermissionDescriptor{
			Id:          permissionID,
			Description: permissions.Describe(permissionID),
		})
	}
	return out
}

func claimString(mc jwt.MapClaims, key string) string {
	v, ok := mc[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return ""
	}
}

func claimStrings(mc jwt.MapClaims, key string) []string {
	v, ok := mc[key]
	if !ok || v == nil {
		return nil
	}
	switch x := v.(type) {
	case []string:
		return uniqueSortedStrings(x)
	case []any:
		out := make([]string, 0, len(x))
		for i := range x {
			s, _ := x[i].(string)
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			out = append(out, s)
		}
		return uniqueSortedStrings(out)
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return nil
		}
		return []string{s}
	default:
		return nil
	}
}

func uniqueSortedStrings(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for i := range in {
		s := strings.TrimSpace(in[i])
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}

func mapKeysSorted(set map[string]struct{}) []string {
	if len(set) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	slices.Sort(out)
	return out
}
