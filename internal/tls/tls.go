package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/netsoft/ztna-wizard/internal/config"
)

type TLSManager struct {
	cfg *config.Config
}

func New(cfg *config.Config) *TLSManager {
	return &TLSManager{cfg: cfg}
}

func (m *TLSManager) GenerateSelfSigned(domain string) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate serial: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   domain,
			Organization: []string{"Netsoft ZTNA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain},
	}
	if ip := net.ParseIP(domain); ip != nil {
		template.IPAddresses = []net.IP{ip}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create cert: %w", err)
	}

	if err := os.MkdirAll(m.cfg.CertDir, 0755); err != nil {
		return err
	}

	certFile, err := os.Create(m.cfg.CertFile("fullchain.pem"))
	if err != nil {
		return err
	}
	defer certFile.Close()
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return err
	}

	keyFile, err := os.Create(m.cfg.CertFile("privkey.pem"))
	if err != nil {
		return err
	}
	defer keyFile.Close()
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		return err
	}

	return nil
}

func (m *TLSManager) SaveCustomCert(certPEM, keyPEM string) error {
	if err := os.MkdirAll(m.cfg.CertDir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(m.cfg.CertFile("fullchain.pem"), []byte(certPEM), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(m.cfg.CertFile("privkey.pem"), []byte(keyPEM), 0600); err != nil {
		return err
	}
	return nil
}

func (m *TLSManager) CertsExist() bool {
	_, errCert := os.Stat(m.cfg.CertFile("fullchain.pem"))
	_, errKey := os.Stat(m.cfg.CertFile("privkey.pem"))
	return errCert == nil && errKey == nil
}
