package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const maximumConfigFileSize = 1 << 20

type rawConfig struct {
	Mode      *Mode         `json:"mode"`
	HTTP      *rawHTTP      `json:"http"`
	Database  *rawDatabase  `json:"database"`
	Log       *rawLog       `json:"log"`
	Telemetry *rawTelemetry `json:"telemetry"`
}

type rawHTTP struct {
	Address           *string `json:"address"`
	ReadHeaderTimeout *string `json:"read_header_timeout"`
	ReadTimeout       *string `json:"read_timeout"`
	WriteTimeout      *string `json:"write_timeout"`
	IdleTimeout       *string `json:"idle_timeout"`
	ShutdownTimeout   *string `json:"shutdown_timeout"`
	MaxHeaderBytes    *int    `json:"max_header_bytes"`
	MaxBodyBytes      *int64  `json:"max_body_bytes"`
}

type rawDatabase struct {
	URL                   *string `json:"url"`
	ConnectTimeout        *string `json:"connect_timeout"`
	HealthTimeout         *string `json:"health_timeout"`
	StatementTimeout      *string `json:"statement_timeout"`
	LockTimeout           *string `json:"lock_timeout"`
	MaxConnectionLifetime *string `json:"max_connection_lifetime"`
	MaxConnectionIdleTime *string `json:"max_connection_idle_time"`
	MinConnections        *int32  `json:"min_connections"`
	MaxConnections        *int32  `json:"max_connections"`
}

type rawLog struct {
	Level *string `json:"level"`
}
type rawTelemetry struct {
	Mode        *string  `json:"mode"`
	Endpoint    *string  `json:"endpoint"`
	ServiceName *string  `json:"service_name"`
	SampleRatio *float64 `json:"sample_ratio"`
}

// Load applies defaults, an optional JSON file, TPMP_ environment variables,
// and command-line flags in increasing precedence. environ is explicit so a
// caller cannot accidentally mix process state into tests.
func Load(args, environ []string) (Config, error) {
	cfg := Defaults()
	name, err := configFileArgument(args)
	if err != nil {
		return Config{}, err
	}
	if name != "" {
		if err := applyFile(&cfg, name); err != nil {
			return Config{}, err
		}
		cfg.ConfigFile = name
	}
	if err := applyEnvironment(&cfg, environ); err != nil {
		return Config{}, err
	}
	if err := applyFlags(&cfg, args); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func configFileArgument(args []string) (string, error) {
	var result string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--config" || arg == "-config" {
			if i+1 >= len(args) {
				return "", errors.New("flag --config requires a value")
			}
			i++
			if result != "" {
				return "", errors.New("flag --config provided more than once")
			}
			result = args[i]
		} else if strings.HasPrefix(arg, "--config=") || strings.HasPrefix(arg, "-config=") {
			if result != "" {
				return "", errors.New("flag --config provided more than once")
			}
			result = strings.SplitN(arg, "=", 2)[1]
		}
	}
	return result, nil
}

func applyFile(cfg *Config, name string) error {
	file, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("open configuration file: %w", err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("configuration file must be a regular file")
	}
	if info.Size() > maximumConfigFileSize {
		return errors.New("configuration file exceeds 1 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumConfigFileSize+1))
	if err != nil || len(data) > maximumConfigFileSize {
		return errors.New("configuration file cannot be read safely")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var raw rawConfig
	if err := decoder.Decode(&raw); err != nil {
		return fmt.Errorf("decode configuration file: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("decode configuration file: multiple JSON values")
		}
		return fmt.Errorf("decode configuration file: %w", err)
	}
	return raw.apply(cfg)
}

func (r rawConfig) apply(c *Config) error {
	if r.Mode != nil {
		c.Mode = *r.Mode
	}
	if r.HTTP != nil {
		if r.HTTP.Address != nil {
			c.HTTP.Address = *r.HTTP.Address
		}
		if r.HTTP.MaxHeaderBytes != nil {
			c.HTTP.MaxHeaderBytes = *r.HTTP.MaxHeaderBytes
		}
		if r.HTTP.MaxBodyBytes != nil {
			c.HTTP.MaxBodyBytes = *r.HTTP.MaxBodyBytes
		}
		for name, raw := range map[string]*string{"read_header_timeout": r.HTTP.ReadHeaderTimeout, "read_timeout": r.HTTP.ReadTimeout, "write_timeout": r.HTTP.WriteTimeout, "idle_timeout": r.HTTP.IdleTimeout, "shutdown_timeout": r.HTTP.ShutdownTimeout} {
			if raw != nil {
				d, err := time.ParseDuration(*raw)
				if err != nil {
					return fmt.Errorf("http.%s: invalid duration", name)
				}
				setHTTPDuration(c, name, d)
			}
		}
	}
	if r.Database != nil {
		if r.Database.URL != nil {
			ref, err := ParseSecretRef(*r.Database.URL)
			if err != nil {
				return errors.New("database.url: invalid secret reference")
			}
			c.Database.URL = ref
		}
		if r.Database.MinConnections != nil {
			c.Database.MinConnections = *r.Database.MinConnections
		}
		if r.Database.MaxConnections != nil {
			c.Database.MaxConnections = *r.Database.MaxConnections
		}
		for name, raw := range map[string]*string{"connect_timeout": r.Database.ConnectTimeout, "health_timeout": r.Database.HealthTimeout, "statement_timeout": r.Database.StatementTimeout, "lock_timeout": r.Database.LockTimeout, "max_connection_lifetime": r.Database.MaxConnectionLifetime, "max_connection_idle_time": r.Database.MaxConnectionIdleTime} {
			if raw != nil {
				d, err := time.ParseDuration(*raw)
				if err != nil {
					return fmt.Errorf("database.%s: invalid duration", name)
				}
				setDatabaseDuration(c, name, d)
			}
		}
	}
	if r.Log != nil && r.Log.Level != nil {
		c.Log.Level = *r.Log.Level
	}
	if r.Telemetry != nil {
		if r.Telemetry.Mode != nil {
			c.Telemetry.Mode = *r.Telemetry.Mode
		}
		if r.Telemetry.Endpoint != nil {
			c.Telemetry.Endpoint = *r.Telemetry.Endpoint
		}
		if r.Telemetry.ServiceName != nil {
			c.Telemetry.ServiceName = *r.Telemetry.ServiceName
		}
		if r.Telemetry.SampleRatio != nil {
			c.Telemetry.SampleRatio = *r.Telemetry.SampleRatio
		}
	}
	return nil
}

type setter func(*Config, string) error

var environmentSetters = map[string]setter{
	"TPMP_MODE":                              func(c *Config, v string) error { c.Mode = Mode(v); return nil },
	"TPMP_HTTP_ADDRESS":                      func(c *Config, v string) error { c.HTTP.Address = v; return nil },
	"TPMP_HTTP_READ_HEADER_TIMEOUT":          durationSetter("http.read_header_timeout", func(c *Config, d time.Duration) { c.HTTP.ReadHeaderTimeout = d }),
	"TPMP_HTTP_READ_TIMEOUT":                 durationSetter("http.read_timeout", func(c *Config, d time.Duration) { c.HTTP.ReadTimeout = d }),
	"TPMP_HTTP_WRITE_TIMEOUT":                durationSetter("http.write_timeout", func(c *Config, d time.Duration) { c.HTTP.WriteTimeout = d }),
	"TPMP_HTTP_IDLE_TIMEOUT":                 durationSetter("http.idle_timeout", func(c *Config, d time.Duration) { c.HTTP.IdleTimeout = d }),
	"TPMP_HTTP_SHUTDOWN_TIMEOUT":             durationSetter("http.shutdown_timeout", func(c *Config, d time.Duration) { c.HTTP.ShutdownTimeout = d }),
	"TPMP_HTTP_MAX_HEADER_BYTES":             intSetter("http.max_header_bytes", 0, func(c *Config, v int64) { c.HTTP.MaxHeaderBytes = int(v) }),
	"TPMP_HTTP_MAX_BODY_BYTES":               intSetter("http.max_body_bytes", 64, func(c *Config, v int64) { c.HTTP.MaxBodyBytes = v }),
	"TPMP_DATABASE_URL":                      secretSetter("database.url", func(c *Config, r SecretRef) { c.Database.URL = r }),
	"TPMP_DATABASE_CONNECT_TIMEOUT":          durationSetter("database.connect_timeout", func(c *Config, d time.Duration) { c.Database.ConnectTimeout = d }),
	"TPMP_DATABASE_HEALTH_TIMEOUT":           durationSetter("database.health_timeout", func(c *Config, d time.Duration) { c.Database.HealthTimeout = d }),
	"TPMP_DATABASE_STATEMENT_TIMEOUT":        durationSetter("database.statement_timeout", func(c *Config, d time.Duration) { c.Database.StatementTimeout = d }),
	"TPMP_DATABASE_LOCK_TIMEOUT":             durationSetter("database.lock_timeout", func(c *Config, d time.Duration) { c.Database.LockTimeout = d }),
	"TPMP_DATABASE_MAX_CONNECTION_LIFETIME":  durationSetter("database.max_connection_lifetime", func(c *Config, d time.Duration) { c.Database.MaxConnectionLifetime = d }),
	"TPMP_DATABASE_MAX_CONNECTION_IDLE_TIME": durationSetter("database.max_connection_idle_time", func(c *Config, d time.Duration) { c.Database.MaxConnectionIdleTime = d }),
	"TPMP_DATABASE_MIN_CONNECTIONS":          intSetter("database.min_connections", 32, func(c *Config, v int64) { c.Database.MinConnections = int32(v) }),
	"TPMP_DATABASE_MAX_CONNECTIONS":          intSetter("database.max_connections", 32, func(c *Config, v int64) { c.Database.MaxConnections = int32(v) }),
	"TPMP_LOG_LEVEL":                         func(c *Config, v string) error { c.Log.Level = v; return nil },
	"TPMP_TELEMETRY_MODE":                    func(c *Config, v string) error { c.Telemetry.Mode = v; return nil },
	"TPMP_TELEMETRY_ENDPOINT":                func(c *Config, v string) error { c.Telemetry.Endpoint = v; return nil },
	"TPMP_TELEMETRY_SERVICE_NAME":            func(c *Config, v string) error { c.Telemetry.ServiceName = v; return nil },
	"TPMP_TELEMETRY_SAMPLE_RATIO": func(c *Config, v string) error {
		n, e := strconv.ParseFloat(v, 64)
		if e != nil {
			return errors.New("telemetry.sample_ratio: invalid number")
		}
		c.Telemetry.SampleRatio = n
		return nil
	},
}

func durationSetter(name string, assign func(*Config, time.Duration)) setter {
	return func(c *Config, v string) error {
		d, e := time.ParseDuration(v)
		if e != nil {
			return fmt.Errorf("%s: invalid duration", name)
		}
		assign(c, d)
		return nil
	}
}
func intSetter(name string, bits int, assign func(*Config, int64)) setter {
	return func(c *Config, v string) error {
		if bits == 0 {
			bits = strconv.IntSize
		}
		n, e := strconv.ParseInt(v, 10, bits)
		if e != nil {
			return fmt.Errorf("%s: invalid integer", name)
		}
		assign(c, n)
		return nil
	}
}
func secretSetter(name string, assign func(*Config, SecretRef)) setter {
	return func(c *Config, v string) error {
		r, e := ParseSecretRef(v)
		if e != nil {
			return fmt.Errorf("%s: invalid secret reference", name)
		}
		assign(c, r)
		return nil
	}
}

func applyEnvironment(c *Config, environ []string) error {
	seen := map[string]bool{}
	for _, item := range environ {
		name, value, found := strings.Cut(item, "=")
		if !found || !strings.HasPrefix(name, "TPMP_") {
			continue
		}
		set, ok := environmentSetters[name]
		if !ok {
			return fmt.Errorf("unknown ThinkPixelMP environment variable %s", name)
		}
		if seen[name] {
			return fmt.Errorf("environment variable %s provided more than once", name)
		}
		seen[name] = true
		if err := set(c, value); err != nil {
			return err
		}
	}
	return nil
}

type flagSetter struct {
	c   *Config
	set setter
}

func (v flagSetter) String() string         { return "" }
func (v flagSetter) Set(value string) error { return v.set(v.c, value) }

func applyFlags(c *Config, args []string) error {
	set := flag.NewFlagSet("thinkpixelmp", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	set.String("config", "", "configuration file")
	for envName, apply := range environmentSetters {
		name := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(envName, "TPMP_"), "_", "-"))
		set.Var(flagSetter{c: c, set: apply}, name, "configuration override")
	}
	if err := set.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}
	if set.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	return nil
}

func setHTTPDuration(c *Config, name string, d time.Duration) {
	switch name {
	case "read_header_timeout":
		c.HTTP.ReadHeaderTimeout = d
	case "read_timeout":
		c.HTTP.ReadTimeout = d
	case "write_timeout":
		c.HTTP.WriteTimeout = d
	case "idle_timeout":
		c.HTTP.IdleTimeout = d
	case "shutdown_timeout":
		c.HTTP.ShutdownTimeout = d
	}
}
func setDatabaseDuration(c *Config, name string, d time.Duration) {
	switch name {
	case "connect_timeout":
		c.Database.ConnectTimeout = d
	case "health_timeout":
		c.Database.HealthTimeout = d
	case "statement_timeout":
		c.Database.StatementTimeout = d
	case "lock_timeout":
		c.Database.LockTimeout = d
	case "max_connection_lifetime":
		c.Database.MaxConnectionLifetime = d
	case "max_connection_idle_time":
		c.Database.MaxConnectionIdleTime = d
	}
}
