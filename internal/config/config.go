// Package config loads and validates process configuration while keeping
// operator-managed secret locations and values out of ordinary output.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

// Mode selects validation rules for disposable development or production.
type Mode string

const (
	ModeDevelopment Mode = "development"
	ModeTest        Mode = "test"
	ModeProduction  Mode = "production"
)

type Config struct {
	Mode       Mode
	HTTP       HTTPConfig
	Database   DatabaseConfig
	OIDC       OIDCConfig
	Log        LogConfig
	Telemetry  TelemetryConfig
	ConfigFile string
}

type OIDCConfig struct {
	Issuer            string                    `json:"issuer,omitempty"`
	Audience          string                    `json:"audience,omitempty"`
	AllowedAlgorithms []string                  `json:"allowed_algorithms,omitempty"`
	ClockSkew         time.Duration             `json:"clock_skew"`
	DiscoveryTimeout  time.Duration             `json:"discovery_timeout"`
	TenantClaim       string                    `json:"tenant_claim,omitempty"`
	PrincipalClaim    string                    `json:"principal_claim,omitempty"`
	TenantMappings    []OIDCTenantMappingConfig `json:"tenant_mappings,omitempty"`
}

type OIDCTenantMappingConfig struct {
	ClaimValue string `json:"claim_value"`
	TenantID   string `json:"tenant_id"`
}

type HTTPConfig struct {
	Address           string        `json:"address"`
	ReadHeaderTimeout time.Duration `json:"read_header_timeout"`
	ReadTimeout       time.Duration `json:"read_timeout"`
	WriteTimeout      time.Duration `json:"write_timeout"`
	IdleTimeout       time.Duration `json:"idle_timeout"`
	ShutdownTimeout   time.Duration `json:"shutdown_timeout"`
	MaxHeaderBytes    int           `json:"max_header_bytes"`
	MaxBodyBytes      int64         `json:"max_body_bytes"`
}

type DatabaseConfig struct {
	URL                   SecretRef
	ConnectTimeout        time.Duration
	HealthTimeout         time.Duration
	StatementTimeout      time.Duration
	LockTimeout           time.Duration
	MaxConnectionLifetime time.Duration
	MaxConnectionIdleTime time.Duration
	MinConnections        int32
	MaxConnections        int32
}

type LogConfig struct {
	Level string `json:"level"`
}

type TelemetryConfig struct {
	Mode        string  `json:"mode"`
	Endpoint    string  `json:"endpoint,omitempty"`
	ServiceName string  `json:"service_name"`
	SampleRatio float64 `json:"sample_ratio"`
}

// Defaults returns local-only, bounded defaults. Persistence and trust-bearing
// values intentionally have no default.
func Defaults() Config {
	return Config{
		Mode: ModeDevelopment,
		HTTP: HTTPConfig{Address: "127.0.0.1:8080", ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second,
			IdleTimeout: 60 * time.Second, ShutdownTimeout: 20 * time.Second,
			MaxHeaderBytes: 1 << 20, MaxBodyBytes: 1 << 20},
		Database: DatabaseConfig{ConnectTimeout: 5 * time.Second, HealthTimeout: 2 * time.Second,
			StatementTimeout: 10 * time.Second, LockTimeout: 2 * time.Second,
			MaxConnectionLifetime: 30 * time.Minute, MaxConnectionIdleTime: 5 * time.Minute,
			MinConnections: 0, MaxConnections: 20},
		OIDC:      OIDCConfig{ClockSkew: 30 * time.Second, DiscoveryTimeout: 5 * time.Second},
		Log:       LogConfig{Level: "info"},
		Telemetry: TelemetryConfig{Mode: "noop", ServiceName: "thinkpixelmp", SampleRatio: 0},
	}
}

func (c Config) Validate() error {
	if c.Mode != ModeDevelopment && c.Mode != ModeTest && c.Mode != ModeProduction {
		return errors.New("mode: must be development, test, or production")
	}
	if _, _, err := net.SplitHostPort(c.HTTP.Address); err != nil {
		return errors.New("http.address: must be a host:port address")
	}
	for name, duration := range map[string]time.Duration{
		"http.read_header_timeout": c.HTTP.ReadHeaderTimeout, "http.read_timeout": c.HTTP.ReadTimeout,
		"http.write_timeout": c.HTTP.WriteTimeout, "http.idle_timeout": c.HTTP.IdleTimeout,
		"http.shutdown_timeout": c.HTTP.ShutdownTimeout, "database.connect_timeout": c.Database.ConnectTimeout,
		"database.health_timeout": c.Database.HealthTimeout, "database.statement_timeout": c.Database.StatementTimeout,
		"database.lock_timeout": c.Database.LockTimeout, "database.max_connection_lifetime": c.Database.MaxConnectionLifetime,
		"database.max_connection_idle_time": c.Database.MaxConnectionIdleTime,
	} {
		if duration <= 0 || duration > 24*time.Hour {
			return fmt.Errorf("%s: must be positive and at most 24h", name)
		}
	}
	if c.HTTP.MaxHeaderBytes < 1024 || c.HTTP.MaxHeaderBytes > 16<<20 {
		return errors.New("http.max_header_bytes: must be between 1 KiB and 16 MiB")
	}
	if c.HTTP.MaxBodyBytes < 1024 || c.HTTP.MaxBodyBytes > 16<<20 {
		return errors.New("http.max_body_bytes: must be between 1 KiB and 16 MiB")
	}
	if c.Database.MinConnections < 0 || c.Database.MaxConnections < 1 || c.Database.MinConnections > c.Database.MaxConnections || c.Database.MaxConnections > 1000 {
		return errors.New("database connections: require 0 <= min <= max <= 1000 and max >= 1")
	}
	if c.Mode == ModeProduction && !c.Database.URL.IsSet() {
		return errors.New("database.url: a secret reference is required in production")
	}
	if err := c.OIDC.validate(); err != nil {
		return err
	}
	if c.Log.Level != "debug" && c.Log.Level != "info" && c.Log.Level != "warn" && c.Log.Level != "error" {
		return errors.New("log.level: must be debug, info, warn, or error")
	}
	if c.Telemetry.Mode != "noop" && c.Telemetry.Mode != "otlp" {
		return errors.New("telemetry.mode: must be noop or otlp")
	}
	if c.Telemetry.ServiceName == "" || len(c.Telemetry.ServiceName) > 128 {
		return errors.New("telemetry.service_name: must contain 1 to 128 characters")
	}
	if c.Telemetry.SampleRatio < 0 || c.Telemetry.SampleRatio > 1 {
		return errors.New("telemetry.sample_ratio: must be between 0 and 1")
	}
	if c.Telemetry.Mode == "otlp" {
		u, err := url.Parse(c.Telemetry.Endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("telemetry.endpoint: OTLP mode requires an HTTP(S) origin without user information, query, or fragment")
		}
	} else if c.Telemetry.Endpoint != "" {
		return errors.New("telemetry.endpoint: must be empty in noop mode")
	}
	return nil
}

func (c OIDCConfig) validate() error {
	if c.ClockSkew < 0 || c.ClockSkew > 5*time.Minute {
		return errors.New("oidc.clock_skew: must be between 0 and 5m")
	}
	if c.DiscoveryTimeout <= 0 || c.DiscoveryTimeout > time.Minute {
		return errors.New("oidc.discovery_timeout: must be positive and at most 1m")
	}
	configured := c.Issuer != "" || c.Audience != "" || len(c.AllowedAlgorithms) != 0 ||
		c.TenantClaim != "" || c.PrincipalClaim != "" || len(c.TenantMappings) != 0
	if !configured {
		return nil
	}
	u, err := url.Parse(c.Issuer)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("oidc.issuer: must be an HTTPS URL without user information, query, or fragment")
	}
	if c.Audience == "" || len(c.Audience) > 512 {
		return errors.New("oidc.audience: must contain 1 to 512 characters")
	}
	if len(c.AllowedAlgorithms) == 0 {
		return errors.New("oidc.allowed_algorithms: at least one algorithm is required")
	}
	seen := make(map[string]struct{}, len(c.AllowedAlgorithms))
	for _, algorithm := range c.AllowedAlgorithms {
		switch algorithm {
		case "RS256", "RS384", "RS512", "PS256", "PS384", "PS512", "ES256", "ES384", "ES512", "EdDSA":
		default:
			return fmt.Errorf("oidc.allowed_algorithms: unsupported algorithm %q", algorithm)
		}
		if _, exists := seen[algorithm]; exists {
			return fmt.Errorf("oidc.allowed_algorithms: duplicate algorithm %q", algorithm)
		}
		seen[algorithm] = struct{}{}
	}
	if err := validateOIDCClaimText(c.TenantClaim, 128); err != nil {
		return fmt.Errorf("oidc.tenant_claim: %w", err)
	}
	if err := validateOIDCClaimText(c.PrincipalClaim, 128); err != nil {
		return fmt.Errorf("oidc.principal_claim: %w", err)
	}
	if len(c.TenantMappings) == 0 || len(c.TenantMappings) > 10000 {
		return errors.New("oidc.tenant_mappings: must contain 1 to 10000 entries")
	}
	claimValues := make(map[string]struct{}, len(c.TenantMappings))
	for index, mapping := range c.TenantMappings {
		if err := validateOIDCClaimText(mapping.ClaimValue, 1024); err != nil {
			return fmt.Errorf("oidc.tenant_mappings[%d].claim_value: %w", index, err)
		}
		if _, duplicate := claimValues[mapping.ClaimValue]; duplicate {
			return fmt.Errorf("oidc.tenant_mappings[%d].claim_value: duplicate value", index)
		}
		claimValues[mapping.ClaimValue] = struct{}{}
		if _, err := shared.ParseUUID(mapping.TenantID); err != nil {
			return fmt.Errorf("oidc.tenant_mappings[%d].tenant_id: %w", index, err)
		}
	}
	return nil
}

func validateOIDCClaimText(value string, maximum int) error {
	if len(value) == 0 || len(value) > maximum || !utf8.ValidString(value) || strings.IndexFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return fmt.Errorf("must contain 1 to %d valid UTF-8 bytes without control characters", maximum)
	}
	return nil
}

type safeSecretReference struct {
	Configured bool         `json:"configured"`
	Source     SecretSource `json:"source,omitempty"`
}

type safeConfig struct {
	Mode     Mode       `json:"mode"`
	HTTP     HTTPConfig `json:"http"`
	Database struct {
		URL                   safeSecretReference `json:"url"`
		ConnectTimeout        time.Duration       `json:"connect_timeout"`
		HealthTimeout         time.Duration       `json:"health_timeout"`
		StatementTimeout      time.Duration       `json:"statement_timeout"`
		LockTimeout           time.Duration       `json:"lock_timeout"`
		MaxConnectionLifetime time.Duration       `json:"max_connection_lifetime"`
		MaxConnectionIdleTime time.Duration       `json:"max_connection_idle_time"`
		MinConnections        int32               `json:"min_connections"`
		MaxConnections        int32               `json:"max_connections"`
	} `json:"database"`
	Log                  LogConfig       `json:"log"`
	OIDC                 safeOIDCConfig  `json:"oidc"`
	Telemetry            TelemetryConfig `json:"telemetry"`
	ConfigFileConfigured bool            `json:"config_file_configured"`
}

type safeOIDCConfig struct {
	Issuer             string        `json:"issuer,omitempty"`
	Audience           string        `json:"audience,omitempty"`
	AllowedAlgorithms  []string      `json:"allowed_algorithms,omitempty"`
	ClockSkew          time.Duration `json:"clock_skew"`
	DiscoveryTimeout   time.Duration `json:"discovery_timeout"`
	TenantClaim        string        `json:"tenant_claim,omitempty"`
	PrincipalClaim     string        `json:"principal_claim,omitempty"`
	TenantMappingCount int           `json:"tenant_mapping_count"`
}

func (c Config) safe() safeConfig {
	var out safeConfig
	out.Mode, out.HTTP, out.Log, out.Telemetry = c.Mode, c.HTTP, c.Log, c.Telemetry
	out.OIDC = safeOIDCConfig{Issuer: c.OIDC.Issuer, Audience: c.OIDC.Audience,
		AllowedAlgorithms: append([]string(nil), c.OIDC.AllowedAlgorithms...), ClockSkew: c.OIDC.ClockSkew,
		DiscoveryTimeout: c.OIDC.DiscoveryTimeout, TenantClaim: c.OIDC.TenantClaim,
		PrincipalClaim: c.OIDC.PrincipalClaim, TenantMappingCount: len(c.OIDC.TenantMappings)}
	out.Database.URL = safeSecretReference{Configured: c.Database.URL.IsSet(), Source: c.Database.URL.Source()}
	out.Database.ConnectTimeout, out.Database.HealthTimeout = c.Database.ConnectTimeout, c.Database.HealthTimeout
	out.Database.StatementTimeout, out.Database.LockTimeout = c.Database.StatementTimeout, c.Database.LockTimeout
	out.Database.MaxConnectionLifetime, out.Database.MaxConnectionIdleTime = c.Database.MaxConnectionLifetime, c.Database.MaxConnectionIdleTime
	out.Database.MinConnections, out.Database.MaxConnections = c.Database.MinConnections, c.Database.MaxConnections
	out.ConfigFileConfigured = c.ConfigFile != ""
	return out
}

func (c Config) MarshalJSON() ([]byte, error) { return json.Marshal(c.safe()) }
func (c Config) String() string {
	b, err := c.MarshalJSON()
	if err != nil {
		return `{"configuration":"unavailable"}`
	}
	return string(b)
}
func (c Config) GoString() string { return c.String() }

var _ fmt.Stringer = Config{}
