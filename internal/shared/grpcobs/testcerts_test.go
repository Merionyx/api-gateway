package grpcobs

import (
	"crypto"
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

// writeTestTLSFiles creates a temp CA + server cert + key + optional client cert for grpcobs TLS tests.
func writeTestTLSFiles(t *testing.T) (dir, serverCert, serverKey, caPEM, clientCert, clientKey string) {
	t.Helper()
	dir = t.TempDir()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-ca"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:         true,
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
	caPEMPath := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(caPEMPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o600); err != nil {
		t.Fatal(err)
	}

	srvKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	srvTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "server"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	srvDER, err := x509.CreateCertificate(rand.Reader, srvTmpl, caCert, srvKey.Public(), caKey)
	if err != nil {
		t.Fatal(err)
	}
	serverCert = filepath.Join(dir, "server.pem")
	serverKey = filepath.Join(dir, "server-key.pem")
	writePEMKey(t, serverKey, srvKey)
	if err := os.WriteFile(serverCert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srvDER}), 0o600); err != nil {
		t.Fatal(err)
	}

	clKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clDER, err := x509.CreateCertificate(rand.Reader, clTmpl, caCert, clKey.Public(), caKey)
	if err != nil {
		t.Fatal(err)
	}
	clientCert = filepath.Join(dir, "client.pem")
	clientKey = filepath.Join(dir, "client-key.pem")
	writePEMKey(t, clientKey, clKey)
	if err := os.WriteFile(clientCert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clDER}), 0o600); err != nil {
		t.Fatal(err)
	}

	return dir, serverCert, serverKey, caPEMPath, clientCert, clientKey
}

func writePEMKey(t *testing.T, path string, key crypto.PrivateKey) {
	t.Helper()
	b, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: b}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
}
