package etcd

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func NewEtcdClient(cfg EtcdConfig) (*clientv3.Client, error) {
	clientConfig := clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
	}

	// TLS settings
	if cfg.TLS.Enabled {
		tlsConfig, err := loadTLSConfig(cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS config: %w", err)
		}
		clientConfig.TLS = tlsConfig
	}

	client, err := clientv3.New(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	return client, nil
}

func loadTLSConfig(cfg EtcdTLSConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, err
	}

	caCert, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}, nil
}
