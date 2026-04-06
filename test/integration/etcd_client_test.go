//go:build integration

package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdEndpoints returns client endpoints for integration tests (GitHub Actions service or local).
func EtcdEndpoints() []string {
	e := strings.TrimSpace(os.Getenv("INTEGRATION_ETCD_ENDPOINT"))
	if e == "" {
		return []string{"127.0.0.1:2379"}
	}
	e = strings.TrimPrefix(e, "http://")
	e = strings.TrimPrefix(e, "https://")
	return []string{e}
}

// NewEtcdClient connects with retries (etcd service may start slightly after the job).
func NewEtcdClient(t *testing.T) *clientv3.Client {
	t.Helper()
	eps := EtcdEndpoints()
	var lastErr error
	for attempt := 1; attempt <= 30; attempt++ {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   eps,
			DialTimeout: 3 * time.Second,
		})
		if err != nil {
			lastErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_, err = cli.Status(ctx, eps[0])
		cancel()
		if err == nil {
			return cli
		}
		lastErr = err
		_ = cli.Close()
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("etcd client: %v", lastErr)
	return nil
}
