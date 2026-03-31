package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"

	"merionyx/api-gateway/internal/auth-sidecar/container"
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

	log.Printf("ExtAuthz server listening on port %s", cnt.Config.Server.GRPCPort)

	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

// Check реализует ext_authz Check метод
func (s *ExtAuthzServer) Check(ctx context.Context, req *authv3.CheckRequest) (*authv3.CheckResponse, error) {
	startTime := time.Now()

	log.Printf("Request received: %+v", req)

	// 1. Извлекаем целевой контракт из пути
	path := req.Attributes.Request.Http.Path
	contractName := extractContractFromPath(path)
	if contractName == "" {
		log.Printf("[AUTH] Unable to determine contract from path: %s", path)
		return denyResponse("Invalid path", 400), nil
	}

	// 3. Извлекаем JWT из заголовка X-App-Token: Bearer <token>
	token := req.Attributes.Request.Http.Headers["x-app-token"]
	if token == "" {
		log.Printf("[AUTH] Missing x-app-token header")
		return denyResponse("Missing x-app-token header", 401), nil
	}

	// 4. Валидируем JWT
	claims, err := s.container.JWTValidator.ValidateToken(token)
	if err != nil {
		log.Printf("[AUTH] Invalid JWT: %v", err)
		return denyResponse("Invalid token", 401), nil
	}

	// 5. Извлекаем app_id и environment из JWT
	appID, ok := claims["app_id"].(string)
	if !ok {
		return denyResponse("Invalid token: missing app_id", 401), nil
	}

	tokenEnv, ok := claims["environment"].(string)
	if !ok {
		return denyResponse("Invalid token: missing environment", 401), nil
	}

	// 6. Проверяем, что environment совпадает
	if tokenEnv != s.container.Config.Controller.Environment {
		return denyResponse(fmt.Sprintf("Token for %s, but current env is %s",
			tokenEnv, s.container.Config.Controller.Environment), 403), nil
	}

	// 7. Проверяем права доступа из in-memory storage
	allowed := s.container.AccessStorage.CheckAccess(contractName, appID, tokenEnv)
	if !allowed {
		log.Printf("[AUTH] Access denied: app=%s contract=%s env=%s", appID, contractName, tokenEnv)
		return denyResponse(fmt.Sprintf("Access denied to contract %s", contractName), 403), nil
	}

	duration := time.Since(startTime)
	log.Printf("[AUTH] ✓ Allowed: app=%s contract=%s env=%s duration=%v",
		appID, contractName, tokenEnv, duration)

	return allowResponse(appID, contractName), nil
}

func extractContractFromPath(path string) string {
	// /api/services/proxy-list-01/v1/users → proxy-list-01
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 3 && parts[0] == "api" && parts[1] == "services" {
		return parts[2]
	}
	return ""
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
				Body: fmt.Sprintf(`{"error": "%s"}`, reason),
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
