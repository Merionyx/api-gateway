package openapi

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/auth/permissions"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	httpauthz "github.com/merionyx/api-gateway/internal/api-server/delivery/http/authz"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/usecase/auth"
)

func (s *StrictOpenAPIServer) AuthorizeOidc(ctx context.Context, request apiserver.AuthorizeOidcRequestObject) (apiserver.AuthorizeOidcResponseObject, error) {
	fc, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}
	tctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	nonce := ""
	if request.Params.Nonce != nil {
		nonce = *request.Params.Nonce
	}

	loc, err := s.c.OIDCLoginUseCase.Start(tctx, auth.OIDCLoginStartRequest{
		ProviderID:          stringOrEmpty(request.Params.ProviderId),
		RedirectURI:         request.Params.RedirectUri,
		ServerCallbackURI:   fc.BaseURL() + "/v1/auth/callback",
		Nonce:               nonce,
		ResponseType:        string(request.Params.ResponseType),
		ClientID:            request.Params.ClientId,
		State:               stringOrEmpty(request.Params.State),
		CodeChallenge:       request.Params.CodeChallenge,
		CodeChallengeMethod: string(request.Params.CodeChallengeMethod),
	})
	if err != nil {
		st, code, detail := auth.MapStartError(err)
		switch st {
		case http.StatusBadRequest:
			p := problem.WithCode(st, code, "", detail)
			return apiserver.AuthorizeOidc400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
			}, nil
		case http.StatusBadGateway:
			p := problem.BadGateway(code, "", detail)
			return apiserver.AuthorizeOidc502ApplicationProblemPlusJSONResponse{
				BadGatewayApplicationProblemPlusJSONResponse: apiserver.BadGatewayApplicationProblemPlusJSONResponse(p),
			}, nil
		case http.StatusServiceUnavailable:
			p := problem.ServiceUnavailable(code, "", detail)
			return apiserver.AuthorizeOidc503ApplicationProblemPlusJSONResponse{
				ServiceUnavailableApplicationProblemPlusJSONResponse: apiserver.ServiceUnavailableApplicationProblemPlusJSONResponse(p),
			}, nil
		default:
			p := internalProblem()
			return apiserver.AuthorizeOidc500ApplicationProblemPlusJSONResponse{
				InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
			}, nil
		}
	}

	return apiserver.AuthorizeOidc302Response{Headers: apiserver.AuthorizeOidc302ResponseHeaders{Location: loc}}, nil
}

func (s *StrictOpenAPIServer) CallbackOidc(ctx context.Context, request apiserver.CallbackOidcRequestObject) (apiserver.CallbackOidcResponseObject, error) {
	tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	out, err := s.c.OIDCCallbackUseCase.CompleteWithResult(tctx, request.Params.Code, request.Params.State)
	if err != nil {
		st, code, detail := auth.MapCallbackError(err)
		switch st {
		case http.StatusBadRequest:
			p := problem.WithCode(st, code, "", detail)
			return apiserver.CallbackOidc400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
			}, nil
		case http.StatusUnauthorized:
			p := problem.WithCode(st, code, "", detail)
			return apiserver.CallbackOidc401ApplicationProblemPlusJSONResponse{
				UnauthorizedApplicationProblemPlusJSONResponse: apiserver.UnauthorizedApplicationProblemPlusJSONResponse(p),
			}, nil
		default:
			p := internalProblem()
			return apiserver.CallbackOidc500ApplicationProblemPlusJSONResponse{
				InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
			}, nil
		}
	}

	if out.RedirectURL == "" {
		p := problem.WithCode(http.StatusInternalServerError, "INTERNAL_ERROR", "", "callback produced no redirect URL")
		return apiserver.CallbackOidc500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	return apiserver.CallbackOidc302Response{Headers: apiserver.CallbackOidc302ResponseHeaders{Location: out.RedirectURL}}, nil
}

func (s *StrictOpenAPIServer) ListOidcProviders(ctx context.Context, request apiserver.ListOidcProvidersRequestObject) (apiserver.ListOidcProvidersResponseObject, error) {
	rows := s.c.OIDCLoginUseCase.ListPublicOIDCProviders()
	out := make([]apiserver.OidcProviderDescriptor, len(rows))
	for i, r := range rows {
		out[i] = apiserver.OidcProviderDescriptor{Id: r.ID, Name: r.Name, Kind: r.Kind, Issuer: r.Issuer}
	}
	return apiserver.ListOidcProviders200JSONResponse{Data: out}, nil
}

func (s *StrictOpenAPIServer) ListAuthPermissions(ctx context.Context, request apiserver.ListAuthPermissionsRequestObject) (apiserver.ListAuthPermissionsResponseObject, error) {
	byID := make(map[string]string)
	for _, d := range permissions.ListDescriptors() {
		byID[d.ID] = d.Description
	}
	for _, roleRow := range s.c.RoleCatalog.ListRolePermissions() {
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
	sort.Strings(ids)

	out := make([]apiserver.PermissionDescriptor, 0, len(ids))
	for _, id := range ids {
		out = append(out, apiserver.PermissionDescriptor{Id: id, Description: byID[id]})
	}
	return apiserver.ListAuthPermissions200JSONResponse{Data: out}, nil
}

func (s *StrictOpenAPIServer) ListAuthRoles(ctx context.Context, request apiserver.ListAuthRolesRequestObject) (apiserver.ListAuthRolesResponseObject, error) {
	roleRows := s.c.RoleCatalog.ListRolePermissions()
	out := make([]apiserver.RolePermissions, 0, len(roleRows))
	for i := range roleRows {
		out = append(out, apiserver.RolePermissions{
			Role:        roleRows[i].RoleID,
			Permissions: permissionDescriptorsFromIDs(roleRows[i].Permissions),
		})
	}
	return apiserver.ListAuthRoles200JSONResponse{Data: out}, nil
}

func (s *StrictOpenAPIServer) TokenOidc(ctx context.Context, request apiserver.TokenOidcRequestObject) (apiserver.TokenOidcResponseObject, error) {
	if s.c.OAuthTokenUseCase == nil {
		return apiserver.TokenOidc500JSONResponse{
			Error:            "server_error",
			ErrorDescription: stringPtr("OAuth token endpoint is not configured."),
		}, nil
	}

	body := request.Body
	if body == nil {
		return apiserver.TokenOidc400JSONResponse{
			Error:            "invalid_request",
			ErrorDescription: stringPtr("request body is required"),
		}, nil
	}

	fc, _ := fiberCtxFromStrictContext(ctx)
	if fc != nil && usesBasicAuthorizationHeader(fc.Get(fiber.HeaderAuthorization)) {
		return apiserver.TokenOidc400JSONResponse{
			Error:            "invalid_request",
			ErrorDescription: stringPtr("Authorization: Basic is not supported on this endpoint; provide client_id in form body."),
		}, nil
	}

	req := auth.OAuthTokenRequest{
		GrantType:    strings.TrimSpace(string(body.GrantType)),
		Code:         stringOrEmpty(body.Code),
		RedirectURI:  stringOrEmpty(body.RedirectUri),
		ClientID:     stringOrEmpty(body.ClientId),
		CodeVerifier: stringOrEmpty(body.CodeVerifier),
		RefreshToken: stringOrEmpty(body.RefreshToken),
		AccessTTL:    durationFromOptionalFormSeconds(body.AccessTtl),
		RefreshTTL:   durationFromOptionalFormSeconds(body.RefreshTtl),
	}

	tctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	out, err := s.c.OAuthTokenUseCase.Exchange(tctx, req)
	if err != nil {
		status, oauthErr, description := auth.MapOAuthTokenError(err)
		resp := apiserver.OAuthTokenError{Error: oauthErr, ErrorDescription: stringPtr(description)}
		switch status {
		case http.StatusBadRequest:
			return apiserver.TokenOidc400JSONResponse(resp), nil
		case http.StatusServiceUnavailable:
			return apiserver.TokenOidc503JSONResponse(resp), nil
		default:
			return apiserver.TokenOidc500JSONResponse(resp), nil
		}
	}

	return apiserver.TokenOidc200JSONResponse(out), nil
}

func (s *StrictOpenAPIServer) InspectTokenPermissions(ctx context.Context, request apiserver.InspectTokenPermissionsRequestObject) (apiserver.InspectTokenPermissionsResponseObject, error) {
	if s.c.JWTUseCase == nil {
		p := internalProblem()
		return apiserver.InspectTokenPermissions500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	if request.Body == nil {
		p := problem.BadRequest(problem.CodeInvalidJSONBody, "", problem.DetailInvalidJSONBody)
		return apiserver.InspectTokenPermissions400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	rawToken := strings.TrimSpace(request.Body.Data.AccessToken)
	if rawToken == "" {
		p := problem.BadRequest("ACCESS_TOKEN_REQUIRED", "", "Field access_token is required.")
		return apiserver.InspectTokenPermissions400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	claims, err := s.c.JWTUseCase.ParseAndValidateAPIProfileBearerToken(rawToken)
	if err != nil {
		p := problem.Unauthorized("INVALID_ACCESS_TOKEN", "", "Provided access token is invalid or expired.")
		return apiserver.InspectTokenPermissions401ApplicationProblemPlusJSONResponse{
			UnauthorizedApplicationProblemPlusJSONResponse: apiserver.UnauthorizedApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	subject := subjectFromAPIJWTClaims(claims)

	tokenRoles := uniqueSortedStrings(httpauthz.NormalizeRolesValue(claims["roles"]))
	effective := s.c.RoleCatalog.ResolvePermissions(tokenRoles)
	for _, permissionID := range httpauthz.ClaimStrings(claims, "permissions") {
		effective[permissionID] = struct{}{}
	}
	for _, permissionID := range httpauthz.ClaimStrings(claims, "scopes") {
		effective[permissionID] = struct{}{}
	}

	permissionIDs := mapKeysSorted(effective)
	return apiserver.InspectTokenPermissions200JSONResponse{Data: apiserver.TokenPermissionsResponse{
		Subject:     subject,
		Roles:       tokenRoles,
		Permissions: permissionDescriptorsFromIDs(permissionIDs),
	}}, nil
}

func (s *StrictOpenAPIServer) IssueApiAccessToken(ctx context.Context, request apiserver.IssueApiAccessTokenRequestObject) (apiserver.IssueApiAccessTokenResponseObject, error) {
	fc, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}

	mc, ok := middleware.APIJWTClaimsFromCtx(fc)
	if !ok {
		p := problem.Forbidden("API_TOKEN_ISSUER_MUST_BE_HUMAN", "", "API access tokens can be issued only by an interactive human Bearer token.")
		return apiserver.IssueApiAccessToken403ApplicationProblemPlusJSONResponse{
			ForbiddenApplicationProblemPlusJSONResponse: apiserver.ForbiddenApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if !hasAnyRoleClaim(mc) {
		p := problem.Forbidden("API_TOKEN_ISSUER_MUST_BE_HUMAN", "", "API access tokens can be issued only by an interactive human Bearer token with role claims.")
		return apiserver.IssueApiAccessToken403ApplicationProblemPlusJSONResponse{
			ForbiddenApplicationProblemPlusJSONResponse: apiserver.ForbiddenApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	have, perr := s.c.PermissionEvaluator.SubjectPermissions(fc)
	if perr != nil {
		p := internalProblem()
		return apiserver.IssueApiAccessToken500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if !httpauthz.HasPermission(have, permissions.APIAccessTokenIssue) {
		p := problem.Forbidden(problem.CodeInsufficientPermissions, "", "The caller does not have any required permission for this operation.")
		return apiserver.IssueApiAccessToken403ApplicationProblemPlusJSONResponse{
			ForbiddenApplicationProblemPlusJSONResponse: apiserver.ForbiddenApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	var requestedPermissions []string
	var requestedExpiresAt *time.Time
	if request.Body != nil {
		requestedPermissions = normalizeRequestedPermissions(request.Body.Data.Permissions)
		requestedExpiresAt = request.Body.Data.ExpiresAt
		if !httpauthz.HasPermission(have, permissions.Wildcard) {
			for i := range requestedPermissions {
				if httpauthz.HasPermission(have, requestedPermissions[i]) {
					continue
				}
				p := problem.Forbidden(problem.CodeRequestedPermissionsNotAllowed, "", "The caller cannot delegate one or more requested permissions.")
				return apiserver.IssueApiAccessToken403ApplicationProblemPlusJSONResponse{
					ForbiddenApplicationProblemPlusJSONResponse: apiserver.ForbiddenApplicationProblemPlusJSONResponse(p),
				}, nil
			}
		}
	}

	subject := subjectFromAPIJWTClaims(mc)
	if subject == "" {
		p := problem.BadRequest("API_TOKEN_SUBJECT_MISSING", "", "Bearer token has no usable sub/email for API access issuance.")
		return apiserver.IssueApiAccessToken400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	now := time.Now().UTC()
	ttl, err := resolveIssuedAPIAccessTTL(now, config.EffectiveInteractiveAccessTokenTTL(s.c.Config.Auth.InteractiveAccessTokenTTL), mc, requestedExpiresAt)
	if err != nil {
		p := problem.BadRequest("API_TOKEN_EXPIRES_AT_INVALID", "", err.Error())
		return apiserver.IssueApiAccessToken400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	requestedAny := stringsToAny(requestedPermissions)
	basePermissions := permissionsFromAPIJWTClaims(mc)
	snap, err := snapshotForAPIAccess(mergeAnyUnique(basePermissions, requestedAny), mc)
	if err != nil {
		p := internalProblem()
		return apiserver.IssueApiAccessToken500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	token, _, exp, err := s.c.JWTUseCase.MintInteractiveAPIAccessJWTFromSnapshot(ctx, subject, snap, ttl)
	if err != nil {
		st, p := mapDomainProblem(err, http.StatusUnauthorized)
		if st == http.StatusUnauthorized {
			return apiserver.IssueApiAccessToken401ApplicationProblemPlusJSONResponse{
				UnauthorizedApplicationProblemPlusJSONResponse: apiserver.UnauthorizedApplicationProblemPlusJSONResponse(p),
			}, nil
		}
		return apiserver.IssueApiAccessToken500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	out := apiserver.ApiAccessTokenIssued{AccessToken: token, ExpiresAt: exp}
	return apiserver.IssueApiAccessToken201JSONResponse{Data: out}, nil
}

func (s *StrictOpenAPIServer) IssueEdgeToken(ctx context.Context, request apiserver.IssueEdgeTokenRequestObject) (apiserver.IssueEdgeTokenResponseObject, error) {
	if request.Body == nil {
		p := problem.BadRequest(problem.CodeInvalidJSONBody, "", problem.DetailInvalidJSONBody)
		return apiserver.IssueEdgeToken400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	fc, err := fiberCtxFromStrictContext(ctx)
	if err != nil {
		return nil, err
	}

	have, perr := s.c.PermissionEvaluator.SubjectPermissions(fc)
	if perr != nil {
		p := internalProblem()
		return apiserver.IssueEdgeToken500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if !httpauthz.HasPermission(have, permissions.EdgeTokenIssue) {
		p := problem.Forbidden(problem.CodeInsufficientPermissions, "", "The caller does not have any required permission for this operation.")
		return apiserver.IssueEdgeToken403ApplicationProblemPlusJSONResponse{
			ForbiddenApplicationProblemPlusJSONResponse: apiserver.ForbiddenApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	req := models.GenerateTokenRequest{
		AppID:        request.Body.Data.AppId,
		Environments: request.Body.Data.Environments,
		ExpiresAt:    request.Body.Data.ExpiresAt,
	}
	if req.AppID == "" {
		p := problem.BadRequest(problem.CodeTokenAppIDRequired, "", problem.DetailTokenAppIDRequired)
		return apiserver.IssueEdgeToken400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	if len(req.Environments) == 0 {
		p := problem.BadRequest(problem.CodeTokenEnvironmentsRequired, "", problem.DetailTokenEnvironmentsRequired)
		return apiserver.IssueEdgeToken400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	for _, env := range req.Environments {
		if env == "" {
			p := problem.BadRequest(problem.CodeTokenEnvironmentEmpty, "", problem.DetailTokenEnvironmentEmpty)
			return apiserver.IssueEdgeToken400ApplicationProblemPlusJSONResponse{
				BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
			}, nil
		}
	}
	if req.ExpiresAt.Before(time.Now()) {
		p := problem.BadRequest(problem.CodeTokenExpiresAtPast, "", problem.DetailTokenExpiresAtPast)
		return apiserver.IssueEdgeToken400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: apiserver.BadRequestApplicationProblemPlusJSONResponse(p),
		}, nil
	}

	token, err := s.c.JWTUseCase.GenerateToken(ctx, &req)
	if err != nil {
		_, p := mapDomainProblem(err)
		return apiserver.IssueEdgeToken500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse: apiserver.InternalErrorApplicationProblemPlusJSONResponse(p),
		}, nil
	}
	return apiserver.IssueEdgeToken201JSONResponse{Data: apiserver.GenerateTokenResponse{
		Id:           token.ID,
		Token:        token.Token,
		AppId:        token.AppID,
		Environments: token.Environments,
		ExpiresAt:    token.ExpiresAt,
		CreatedAt:    token.CreatedAt,
	}}, nil
}
