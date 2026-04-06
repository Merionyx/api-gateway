package etcd

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeClientTLSFiles(t *testing.T) (certFile, keyFile, caFile string) {
	t.Helper()
	dir := t.TempDir()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, caKey.Public(), caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	caFile = filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o600); err != nil {
		t.Fatal(err)
	}

	clKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "etcd-client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clDER, err := x509.CreateCertificate(rand.Reader, clTmpl, caCert, clKey.Public(), caKey)
	if err != nil {
		t.Fatal(err)
	}
	certFile = filepath.Join(dir, "client.pem")
	keyFile = filepath.Join(dir, "client-key.pem")
	pk8, err := x509.MarshalPKCS8PrivateKey(clKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pk8}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile, caFile
}

func TestNewEtcdClient_TLSValidFiles(t *testing.T) {
	cert, key, ca := writeClientTLSFiles(t)
	cli, err := NewEtcdClient(EtcdConfig{
		Endpoints:   []string{"http://127.0.0.1:2379"},
		DialTimeout: 2 * time.Second,
		TLS: EtcdTLSConfig{
			Enabled:  true,
			CertFile: cert,
			KeyFile:  key,
			CAFile:   ca,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cli.Close() })
}

func TestNewEtcdClient_TLSInvalidKeyPEM(t *testing.T) {
	cert, _, ca := writeClientTLSFiles(t)
	badKey := filepath.Join(t.TempDir(), "bad.key")
	if err := os.WriteFile(badKey, []byte("not-pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewEtcdClient(EtcdConfig{
		Endpoints:   []string{"http://127.0.0.1:2379"},
		DialTimeout: time.Second,
		TLS: EtcdTLSConfig{
			Enabled:  true,
			CertFile: cert,
			KeyFile:  badKey,
			CAFile:   ca,
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewEtcdClient_TLSMissingCAFile(t *testing.T) {
	cert, key, _ := writeClientTLSFiles(t)
	_, err := NewEtcdClient(EtcdConfig{
		Endpoints:   []string{"http://127.0.0.1:2379"},
		DialTimeout: time.Second,
		TLS: EtcdTLSConfig{
			Enabled:  true,
			CertFile: cert,
			KeyFile:  key,
			CAFile:   filepath.Join(t.TempDir(), "missing-ca.pem"),
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewEtcdClient_TLSInvalidCertPEM(t *testing.T) {
	_, key, ca := writeClientTLSFiles(t)
	badCert := filepath.Join(t.TempDir(), "bad.crt")
	if err := os.WriteFile(badCert, []byte("not-pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewEtcdClient(EtcdConfig{
		Endpoints:   []string{"http://127.0.0.1:2379"},
		DialTimeout: time.Second,
		TLS: EtcdTLSConfig{
			Enabled:  true,
			CertFile: badCert,
			KeyFile:  key,
			CAFile:   ca,
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
