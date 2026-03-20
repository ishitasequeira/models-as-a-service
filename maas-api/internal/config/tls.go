package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"os"

	"k8s.io/utils/env"
)

const (
	tlsVersion12 = "1.2"
	tlsVersion13 = "1.3"
)

type TLSVersion uint16

var _ flag.Value = (*TLSVersion)(nil)

func (v *TLSVersion) String() string {
	switch uint16(*v) {
	case tls.VersionTLS12:
		return tlsVersion12
	case tls.VersionTLS13:
		return tlsVersion13
	default:
		return tlsVersion12
	}
}

func (v *TLSVersion) Set(s string) error {
	switch s {
	case tlsVersion12:
		*v = TLSVersion(tls.VersionTLS12)
	case tlsVersion13:
		*v = TLSVersion(tls.VersionTLS13)
	default:
		return fmt.Errorf("unsupported TLS version %q: must be %s or %s", s, tlsVersion12, tlsVersion13)
	}
	return nil
}

func (v *TLSVersion) Value() uint16 {
	return uint16(*v)
}

// TLSConfig holds TLS-related configuration.
type TLSConfig struct {
	Cert       string     // Path to TLS certificate
	Key        string     // Path to TLS private key
	SelfSigned bool       // Generate self-signed certificate
	MinVersion TLSVersion // Minimum TLS version

	// ClientCAFile is the path to a CA certificate bundle for outbound TLS connections.
	// If empty, uses system trust store. Common paths:
	// - /var/run/secrets/kubernetes.io/serviceaccount/ca.crt (Kubernetes service CA)
	// - /etc/pki/tls/certs/ca-bundle.crt (RHEL/OpenShift)
	ClientCAFile string

	// ClientInsecureSkipVerify disables TLS verification for outbound connections.
	// WARNING: Only for development/debugging. Not FIPS compliant.
	ClientInsecureSkipVerify bool
}

// Enabled returns true if TLS is configured (either with certs or self-signed).
func (t *TLSConfig) Enabled() bool {
	return t.HasCerts() || t.SelfSigned
}

// HasCerts returns true if certificate files are configured.
func (t *TLSConfig) HasCerts() bool {
	return t.Cert != "" && t.Key != ""
}

// loadTLSConfig loads TLS configuration from environment variables.
func loadTLSConfig() TLSConfig {
	selfSigned, _ := env.GetBool("TLS_SELF_SIGNED", false)
	clientInsecure, _ := env.GetBool("CLIENT_INSECURE_SKIP_VERIFY", false)
	return TLSConfig{
		Cert:                     env.GetString("TLS_CERT", ""),
		Key:                      env.GetString("TLS_KEY", ""),
		SelfSigned:               selfSigned,
		MinVersion:               TLSVersion(tls.VersionTLS12),
		ClientCAFile:             env.GetString("CLIENT_CA_FILE", ""),
		ClientInsecureSkipVerify: clientInsecure,
	}
}

// bindFlags binds TLS flags to the flagset.
func (t *TLSConfig) bindFlags(fs *flag.FlagSet) {
	fs.StringVar(&t.Cert, "tls-cert", t.Cert, "Path to TLS certificate")
	fs.StringVar(&t.Key, "tls-key", t.Key, "Path to TLS private key")
	fs.BoolVar(&t.SelfSigned, "tls-self-signed", t.SelfSigned, "Generate self-signed certificate")
	fs.Var(&t.MinVersion, "tls-min-version", "Minimum TLS version: 1.2 or 1.3 (default: 1.2)")
	fs.StringVar(&t.ClientCAFile, "client-ca-file", t.ClientCAFile, "Path to CA certificate bundle for outbound TLS connections")
	fs.BoolVar(&t.ClientInsecureSkipVerify, "client-insecure-skip-verify", t.ClientInsecureSkipVerify, "Disable TLS verification for outbound connections (not FIPS compliant)")
}

// validate validates TLS configuration.
func (t *TLSConfig) validate() error {
	// Validate that cert and key are provided together
	if (t.Cert != "" && t.Key == "") || (t.Cert == "" && t.Key != "") {
		return errors.New("--tls-cert and --tls-key must both be provided together")
	}

	if t.HasCerts() {
		t.SelfSigned = false
	}

	if envVal := env.GetString("TLS_MIN_VERSION", ""); envVal != "" {
		if err := t.MinVersion.Set(envVal); err != nil {
			return err
		}
	}

	return nil
}

// BuildClientTLSConfig creates a *tls.Config for outbound HTTP client connections.
// It uses the system trust store as a base and optionally appends certificates from ClientCAFile.
// If ClientInsecureSkipVerify is true, certificate validation is disabled (not FIPS compliant).
func (t *TLSConfig) BuildClientTLSConfig() (*tls.Config, error) {
	if t.ClientInsecureSkipVerify {
		//nolint:gosec // G402: Explicit opt-in for development/debugging only
		return &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		}, nil
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}

	if t.ClientCAFile != "" {
		caCert, err := os.ReadFile(t.ClientCAFile)
		if err != nil {
			return nil, fmt.Errorf("reading CA file %s: %w", t.ClientCAFile, err)
		}
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate from %s", t.ClientCAFile)
		}
	}

	return &tls.Config{
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12,
	}, nil
}
