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

	"merionyx/api-gateway/internal/auth-sidecar/container"
	"merionyx/api-gateway/internal/shared/utils"
)

type ExtAuthzServer struct {
	authv3.UnimplementedAuthorizationServer
	container *container.Container
}

func NewExtAuthzServer(cnt *container.Container) *ExtAuthzServer {
	return &ExtAuthzServer{container: cnt}
}

func StartExtAuthzServer(cnt *container.Container) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cnt.Config.Server.GRPCPort))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer()
	extAuthzServer := NewExtAuthzServer(cnt)

	authv3.RegisterAuthorizationServer(grpcServer, extAuthzServer)

	reflection.Register(grpcServer)

	slog.Info("ext_authz server listening", "port", cnt.Config.Server.GRPCPort)

	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

// Check implements the ext_authz Check method
func (s *ExtAuthzServer) Check(ctx context.Context, req *authv3.CheckRequest) (*authv3.CheckResponse, error) {
	startTime := time.Now()

	// 1. Find the contract by path (longest prefix match)
	path := req.Attributes.Request.Http.Path
	accessConfig := s.container.AccessStorage.FindContractByPath(path)

	if accessConfig == nil {
		slog.Warn("auth: contract not found for path", "path", path)
		return denyResponse("Contract not found", 404), nil
	}

	contractName := accessConfig.ContractName

	// 3. Check if the contract is secure
	if !accessConfig.Secure {
		return allowResponse(contractName, contractName), nil
	}

	// 4. Extract the JWT from the X-App-Token: Bearer <token> header
	token := req.Attributes.Request.Http.Headers["x-app-token"]
	if token == "" {
		slog.Warn("auth: missing x-app-token header")
		return denyResponse("Missing x-app-token header", 401), nil
	}

	// 5. Validate the JWT
	claims, err := s.container.JWTValidator.ValidateToken(token)
	if err != nil {
		slog.Warn("auth: invalid JWT", "error", err)
		return denyResponse("Invalid token", 401), nil
	}

	// 6. Extract the app_id and environments from the JWT
	appID, ok := claims["app_id"].(string)
	if !ok {
		return denyResponse("Invalid token: missing app_id", 401), nil
	}

	// Extract environments array from JWT
	environmentsRaw, ok := claims["environments"]
	if !ok {
		return denyResponse("Invalid token: missing environments", 401), nil
	}

	// Convert to string slice
	environmentsInterface, ok := environmentsRaw.([]interface{})
	if !ok {
		return denyResponse("Invalid token: environments must be an array", 401), nil
	}

	var tokenEnvironments []string
	for _, env := range environmentsInterface {
		envStr, ok := env.(string)
		if !ok {
			return denyResponse("Invalid token: environment must be a string", 401), nil
		}
		tokenEnvironments = append(tokenEnvironments, envStr)
	}

	// 7. Check that the current environment matches any of the patterns in the token
	currentEnv := s.container.Config.Controller.Environment
	if !utils.MatchesAnyEnvironment(currentEnv, tokenEnvironments) {
		return denyResponse(fmt.Sprintf("Token environments %v don't match current env %s",
			tokenEnvironments, currentEnv), 403), nil
	}

	// 8. Check access rights from in-memory storage
	allowed := s.container.AccessStorage.CheckAccess(contractName, appID, currentEnv)
	if !allowed {
		slog.Warn("auth: access denied", "app_id", appID, "contract", contractName, "environment", currentEnv)
		return denyResponse(fmt.Sprintf("Access denied to contract %s", contractName), 403), nil
	}

	duration := time.Since(startTime)
	slog.Info("auth: allowed",
		"app_id", appID, "contract", contractName, "environment", currentEnv,
		"env_patterns", tokenEnvironments, "duration", duration)

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
