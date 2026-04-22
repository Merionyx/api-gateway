package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"

	"github.com/merionyx/api-gateway/internal/auth-sidecar/container"
	authmetrics "github.com/merionyx/api-gateway/internal/auth-sidecar/metrics"
	"github.com/merionyx/api-gateway/internal/shared/grpcobs"
	"github.com/merionyx/api-gateway/internal/shared/serviceapp"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"
	"github.com/merionyx/api-gateway/internal/shared/utils"
)

type ExtAuthzServer struct {
	authv3.UnimplementedAuthorizationServer
	container *container.Container
}

func NewExtAuthzServer(cnt *container.Container) *ExtAuthzServer {
	return &ExtAuthzServer{container: cnt}
}

// RunExtAuthzServer serves ext_authz until ctx is cancelled.
func RunExtAuthzServer(ctx context.Context, cnt *container.Container) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cnt.Config.Server.GRPCPort))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	opts, err := grpcobs.ServerOptions(&cnt.Config.GRPCExtAuthz.TLS, cnt.Config.GRPCExtAuthz.Observability, cnt.Config.MetricsHTTP.Enabled)
	if err != nil {
		return fmt.Errorf("ext_authz gRPC options: %w", err)
	}
	grpcSrv := grpc.NewServer(opts...)
	extAuthzServer := NewExtAuthzServer(cnt)

	authv3.RegisterAuthorizationServer(grpcSrv, extAuthzServer)

	if cnt.Config.GRPCExtAuthz.Observability.ReflectionEnabled {
		reflection.Register(grpcSrv)
	}

	slog.Info("ext_authz server listening", "port", cnt.Config.Server.GRPCPort)
	return serviceapp.RunGRPCServeUntil(ctx, grpcSrv, lis)
}

// Check implements the ext_authz Check method
func (s *ExtAuthzServer) Check(ctx context.Context, req *authv3.CheckRequest) (*authv3.CheckResponse, error) {
	ctx, span := telemetry.ServerSpan(ctx, spanExtAuthzPkg, "Check")
	defer span.End()

	startTime := time.Now()
	enabled := s.container.Config.MetricsHTTP.Enabled
	record := func(result, reason string) {
		authmetrics.RecordAuthorization(enabled, result, reason, time.Since(startTime))
	}

	// 1. Find the contract by path (longest prefix match)
	path := req.Attributes.Request.Http.Path
	_, findSp := telemetry.Start(ctx, telemetry.SpanName(spanExtAuthzPkg, "accessStorage.findContractByPath"))
	accessConfig := s.container.AccessStorage.FindContractByPath(path)
	findSp.End()

	if accessConfig == nil {
		slog.Warn("auth: contract not found for path", "path", path)
		record(authmetrics.ResultDeny, authmetrics.ReasonContractNotFound)
		return denyResponse("Contract not found", 404), nil
	}

	contractName := accessConfig.ContractName

	// 3. Check if the contract is secure
	if !accessConfig.Secure {
		record(authmetrics.ResultAllow, authmetrics.ReasonInsecureAllow)
		return allowResponse(contractName, contractName), nil
	}

	// 4. Extract the JWT from the X-App-Token: Bearer <token> header
	token := req.Attributes.Request.Http.Headers["x-app-token"]
	if token == "" {
		slog.Warn("auth: missing x-app-token header")
		record(authmetrics.ResultDeny, authmetrics.ReasonMissingToken)
		return denyResponse("Missing x-app-token header", 401), nil
	}

	// 5. Validate the JWT
	_, jwtSp := telemetry.Start(ctx, telemetry.SpanName(spanExtAuthzPkg, "jwtValidator.validateToken"))
	claims, err := s.container.JWTValidator.ValidateToken(token)
	if err != nil {
		telemetry.MarkError(jwtSp, err)
		jwtSp.End()
		slog.Warn("auth: invalid JWT", "error", err)
		record(authmetrics.ResultDeny, authmetrics.ReasonInvalidJWT)
		return denyResponse("Invalid token", 401), nil
	}
	jwtSp.End()

	// 6. Extract the app_id and environments from the JWT
	appID, ok := claims["app_id"].(string)
	if !ok {
		record(authmetrics.ResultDeny, authmetrics.ReasonMissingAppID)
		return denyResponse("Invalid token: missing app_id", 401), nil
	}

	// Extract environments array from JWT
	environmentsRaw, ok := claims["environments"]
	if !ok {
		record(authmetrics.ResultDeny, authmetrics.ReasonMissingEnvironments)
		return denyResponse("Invalid token: missing environments", 401), nil
	}

	// Convert to string slice
	environmentsInterface, ok := environmentsRaw.([]interface{})
	if !ok {
		record(authmetrics.ResultDeny, authmetrics.ReasonEnvironmentsWrongType)
		return denyResponse("Invalid token: environments must be an array", 401), nil
	}

	var tokenEnvironments []string
	for _, env := range environmentsInterface {
		envStr, ok := env.(string)
		if !ok {
			record(authmetrics.ResultDeny, authmetrics.ReasonEnvironmentNotString)
			return denyResponse("Invalid token: environment must be a string", 401), nil
		}
		tokenEnvironments = append(tokenEnvironments, envStr)
	}

	// 7. Check that the current environment matches any of the patterns in the token
	currentEnv := s.container.Config.Controller.Environment
	_, envSp := telemetry.Start(ctx, telemetry.SpanName(spanExtAuthzPkg, "authorize.checkEnvironmentAndAccess"))
	if !utils.MatchesAnyEnvironment(currentEnv, tokenEnvironments) {
		envSp.End()
		record(authmetrics.ResultDeny, authmetrics.ReasonEnvNotAllowed)
		return denyResponse(fmt.Sprintf("Token environments %v don't match current env %s",
			tokenEnvironments, currentEnv), 403), nil
	}

	// 8. Check access rights from in-memory storage
	allowed := s.container.AccessStorage.CheckAccess(contractName, appID, currentEnv)
	if !allowed {
		envSp.End()
		slog.Warn("auth: access denied", "app_id", appID, "contract", contractName, "environment", currentEnv)
		record(authmetrics.ResultDeny, authmetrics.ReasonAccessDenied)
		return denyResponse(fmt.Sprintf("Access denied to contract %s", contractName), 403), nil
	}
	envSp.End()

	duration := time.Since(startTime)
	slog.Info("auth: allowed",
		"app_id", appID, "contract", contractName, "environment", currentEnv,
		"env_patterns", tokenEnvironments, "duration", duration)

	record(authmetrics.ResultAllow, authmetrics.ReasonAllowOK)
	return allowResponse(appID, contractName), nil
}

func allowResponse(appID, contractName string) *authv3.CheckResponse {
	return &authv3.CheckResponse{
		Status: &status.Status{Code: int32(codes.OK)},
		HttpResponse: &authv3.CheckResponse_OkResponse{
			OkResponse: &authv3.OkHttpResponse{
				Headers: []*corev3.HeaderValueOption{
					{
						Header: &corev3.HeaderValue{
							Key:   "x-app-id",
							Value: appID,
						},
					},
					{
						Header: &corev3.HeaderValue{
							Key:   "x-contract",
							Value: contractName,
						},
					},
				},
			},
		},
	}
}

func denyResponse(reason string, statusCode int) *authv3.CheckResponse {
	var envoyStatus typev3.StatusCode
	switch statusCode {
	case 400:
		envoyStatus = typev3.StatusCode_BadRequest
	case 401:
		envoyStatus = typev3.StatusCode_Unauthorized
	case 403:
		envoyStatus = typev3.StatusCode_Forbidden
	default:
		envoyStatus = typev3.StatusCode_Unauthorized
	}

	bodyBytes, err := json.Marshal(map[string]string{"error": reason})
	if err != nil {
		bodyBytes = []byte(`{"error":"internal error"}`)
	}

	return &authv3.CheckResponse{
		Status: &status.Status{
			Code:    int32(codes.PermissionDenied),
			Message: reason,
		},
		HttpResponse: &authv3.CheckResponse_DeniedResponse{
			DeniedResponse: &authv3.DeniedHttpResponse{
				Status: &typev3.HttpStatus{
					Code: envoyStatus,
				},
				Body: string(bodyBytes),
				Headers: []*corev3.HeaderValueOption{
					{
						Header: &corev3.HeaderValue{
							Key:   "content-type",
							Value: "application/json",
						},
					},
				},
			},
		},
	}
}
