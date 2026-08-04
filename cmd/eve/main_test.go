package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHelpDocumentsGardenInstallation(t *testing.T) {
	for _, argument := range []string{"help", "--help", "-h"} {
		t.Run(argument, func(t *testing.T) {
			var output strings.Builder
			if err := run([]string{argument}, &output); err != nil {
				t.Fatal(err)
			}
			for _, required := range []string{
				"garden <command> [options]",
				"make install",
				"$HOME/.local/bin/garden",
				"command -v garden",
				"garden version",
			} {
				if !strings.Contains(output.String(), required) {
					t.Fatalf("help omits %q", required)
				}
			}
		})
	}
}

func TestUnknownCommandReturnsGardenUsage(t *testing.T) {
	err := run([]string{"unknown"}, io.Discard)
	if err == nil {
		t.Fatal("unknown command succeeded")
	}
	for _, required := range []string{`unknown command "unknown"`, "garden <command> [options]"} {
		if !strings.Contains(err.Error(), required) {
			t.Fatalf("error omits %q: %v", required, err)
		}
	}
}

func TestHelpReportsOutputFailure(t *testing.T) {
	err := run([]string{"help"}, failingWriter{})
	if err == nil || !strings.Contains(err.Error(), "write help") {
		t.Fatalf("error = %v", err)
	}
}

func TestCommandHelpSucceeds(t *testing.T) {
	for _, command := range []string{"init", "info", "run", "serve", "eval"} {
		t.Run(command, func(t *testing.T) {
			if err := run([]string{command, "--help"}, io.Discard); err != nil {
				t.Fatalf("%s --help: %v", command, err)
			}
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestAuthenticatedHandlerRequiresTokenForPublicBind(t *testing.T) {
	_, err := authenticatedHandler("0.0.0.0:3000", "", http.NotFoundHandler())
	if err == nil || !strings.Contains(err.Error(), "GARDEN_AUTH_TOKEN") {
		t.Fatalf("error = %v", err)
	}
}

func TestServeDefaultsToLoopback(t *testing.T) {
	options, err := parseServeOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	if options.addr != "127.0.0.1:3000" {
		t.Fatalf("default address = %q", options.addr)
	}
	if options.runtime != "native" {
		t.Fatalf("default runtime = %q", options.runtime)
	}
}

func TestServeSelectsOfficialEveRuntimeExplicitly(t *testing.T) {
	options, err := parseServeOptions([]string{"--runtime", "eve", "--root", "/tmp/agent"})
	if err != nil {
		t.Fatal(err)
	}
	if options.runtime != "eve" || options.root != "/tmp/agent" {
		t.Fatalf("options = %+v", options)
	}
}

func TestAuthenticatedHandlerProtectsPublicBind(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler, err := authenticatedHandler("0.0.0.0:3000", "secret", next)
	if err != nil {
		t.Fatal(err)
	}

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/eve/v1/health", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}

	authorized := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/eve/v1/health", nil)
	request.Header.Set("Authorization", "Bearer secret")
	handler.ServeHTTP(authorized, request)
	if authorized.Code != http.StatusNoContent {
		t.Fatalf("authorized status = %d", authorized.Code)
	}
}

func TestAuthenticatedHandlerLeavesLoopbackLocal(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	for _, addr := range []string{"127.0.0.1:3000", "[::1]:3000", "localhost:3000"} {
		handler, err := authenticatedHandler(addr, "", next)
		if err != nil {
			t.Fatalf("%s: %v", addr, err)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		if response.Code != http.StatusNoContent {
			t.Fatalf("%s status = %d", addr, response.Code)
		}
	}
}

func TestConfiguredTokenAlsoProtectsLoopback(t *testing.T) {
	handler, err := authenticatedHandler("127.0.0.1:3000", "secret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	if err != nil {
		t.Fatal(err)
	}
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}
	authorized := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer secret")
	handler.ServeHTTP(authorized, request)
	if authorized.Code != http.StatusNoContent {
		t.Fatalf("authorized status = %d", authorized.Code)
	}
}
