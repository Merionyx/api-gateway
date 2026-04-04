package grpcobs

import (
	"fmt"

	"google.golang.org/grpc"
)

// DialOptions returns grpc.WithTransportCredentials for NewClient.
func DialOptions(tls ClientTLSConfig) ([]grpc.DialOption, error) {
	creds, err := ClientTransportCredentials(tls)
	if err != nil {
		return nil, fmt.Errorf("grpc client tls: %w", err)
	}
	return []grpc.DialOption{grpc.WithTransportCredentials(creds)}, nil
}
