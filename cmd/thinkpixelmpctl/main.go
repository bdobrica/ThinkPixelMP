package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultEndpoint = "http://127.0.0.1:8080"
	maxResponseSize = 1 << 20
	usage           = "usage: thinkpixelmpctl [--endpoint URL] [--timeout DURATION] [--token-env NAME] <live|ready>"
)

type client struct {
	endpoint *url.URL
	token    string
	http     *http.Client
}

var newHTTPClient = func(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "thinkpixelmpctl:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, stdout io.Writer) error {
	flags := flag.NewFlagSet("thinkpixelmpctl", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	endpoint := flags.String("endpoint", envOrDefault(getenv, "TPMPCTL_ENDPOINT", defaultEndpoint), "ThinkPixelMP API origin")
	timeout := flags.Duration("timeout", 15*time.Second, "request timeout")
	tokenEnv := flags.String("token-env", "TPMPCTL_TOKEN", "environment variable containing a bearer token")
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		_, err = fmt.Fprintln(stdout, usage)
		return err
	} else if err != nil {
		return usageError(err)
	}
	if *timeout <= 0 || *timeout > 5*time.Minute {
		return errors.New("timeout must be positive and at most 5m")
	}
	if !validEnvironmentName(*tokenEnv) {
		return errors.New("token-env must be a valid environment variable name")
	}
	commandArgs := flags.Args()
	if len(commandArgs) != 1 {
		return usageError(nil)
	}
	path, ok := map[string]string{"live": "/livez", "ready": "/readyz"}[commandArgs[0]]
	if !ok {
		return usageError(fmt.Errorf("unknown command %q", commandArgs[0]))
	}

	base, err := validateEndpoint(*endpoint)
	if err != nil {
		return err
	}
	api := client{endpoint: base, token: getenv(*tokenEnv), http: newHTTPClient(*timeout)}
	body, err := api.get(ctx, path)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, string(body))
	return err
}

func (c client) get(ctx context.Context, path string) ([]byte, error) {
	target := *c.endpoint
	target.Path = strings.TrimSuffix(target.Path, "/") + path
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create API request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "thinkpixelmpctl/dev")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call API: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("read API response: %w", err)
	}
	if len(body) > maxResponseSize {
		return nil, errors.New("API response exceeds 1 MiB limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned %s", response.Status)
	}
	return body, nil
}

func validateEndpoint(value string) (*url.URL, error) {
	endpoint, err := url.Parse(value)
	if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || (endpoint.Path != "" && endpoint.Path != "/") || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return nil, errors.New("endpoint must be an HTTP(S) origin without credentials, query, or fragment")
	}
	host := endpoint.Hostname()
	address := net.ParseIP(host)
	if endpoint.Scheme == "http" && host != "localhost" && (address == nil || !address.IsLoopback()) {
		return nil, errors.New("non-loopback endpoints require HTTPS")
	}
	return endpoint, nil
}

func validEnvironmentName(value string) bool {
	if value == "" || !isEnvironmentNameStart(value[0]) {
		return false
	}
	for i := 1; i < len(value); i++ {
		if !isEnvironmentNameStart(value[i]) && (value[i] < '0' || value[i] > '9') {
			return false
		}
	}
	return true
}

func isEnvironmentNameStart(value byte) bool {
	return value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func envOrDefault(getenv func(string) string, name, fallback string) string {
	if value := getenv(name); value != "" {
		return value
	}
	return fallback
}

func usageError(cause error) error {
	if cause == nil {
		return errors.New(usage)
	}
	return fmt.Errorf("%v; %s", cause, usage)
}
