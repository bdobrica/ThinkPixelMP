package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func useTransport(t *testing.T, transport roundTripFunc) {
	t.Helper()
	previous := newHTTPClient
	newHTTPClient = func(timeout time.Duration) *http.Client {
		return &http.Client{Timeout: timeout, Transport: transport}
	}
	t.Cleanup(func() { newHTTPClient = previous })
}

func TestRunCallsAPIWithBearerToken(t *testing.T) {
	useTransport(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/readyz" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("authorization = %q", got)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(`{"status":"ready"}`))}, nil
	})

	values := map[string]string{"TPMPCTL_TOKEN": "secret"}
	var output strings.Builder
	err := run(context.Background(), []string{"ready"}, func(name string) string { return values[name] }, &output)
	if err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "{\"status\":\"ready\"}\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestRunRejectsUnknownCommandWithoutCallingAPI(t *testing.T) {
	err := run(context.Background(), []string{"catalogs"}, func(string) string { return "" }, &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunHelpSucceeds(t *testing.T) {
	var output strings.Builder
	if err := run(context.Background(), []string{"--help"}, func(string) string { return "" }, &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != usage+"\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestValidateEndpointRequiresTLSOffLoopback(t *testing.T) {
	if _, err := validateEndpoint("http://marketplace.example.test"); err == nil {
		t.Fatal("expected insecure remote endpoint to be rejected")
	}
	if _, err := validateEndpoint("https://marketplace.example.test"); err != nil {
		t.Fatalf("HTTPS endpoint: %v", err)
	}
}

func TestClientBoundsResponse(t *testing.T) {
	useTransport(t, func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(strings.Repeat("x", maxResponseSize+1)))}, nil
	})

	var output strings.Builder
	err := run(context.Background(), []string{"live"}, func(string) string { return "" }, &output)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v", err)
	}
}
