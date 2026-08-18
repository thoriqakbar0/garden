package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelpDocumentsGardenInstallation(t *testing.T) {
	for _, argument := range []string{"help", "--help", "-h"} {
		t.Run(argument, func(t *testing.T) {
			var output strings.Builder
			if err := run([]string{argument}, commandStreams{stdout: &output, stderr: io.Discard}); err != nil {
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

func TestInfoUsesCommandStreams(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "agent", "instructions.md"),
		[]byte("Be useful."),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if err := run(
		[]string{"info", "--root", root},
		commandStreams{stdout: &output, stderr: io.Discard},
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"instructions": "Be useful."`) {
		t.Fatalf("info output = %s", output.String())
	}
}

func TestInfoExposesOnlyRootOption(t *testing.T) {
	var help strings.Builder
	if err := run(
		[]string{"info", "--help"},
		commandStreams{stdout: io.Discard, stderr: &help},
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(help.String(), "-root string") || strings.Contains(help.String(), "-addr") {
		t.Fatalf("info help = %q", help.String())
	}
	err := run(
		[]string{"info", "--addr", "127.0.0.1:3000"},
		commandStreams{stdout: io.Discard, stderr: io.Discard},
	)
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -addr") {
		t.Fatalf("info --addr error = %v", err)
	}
}

func TestUnknownCommandReturnsGardenUsage(t *testing.T) {
	err := run([]string{"unknown"}, commandStreams{stdout: io.Discard, stderr: io.Discard})
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
	err := run([]string{"help"}, commandStreams{stdout: failingWriter{}, stderr: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "write help") {
		t.Fatalf("error = %v", err)
	}
}

func TestCommandHelpSucceeds(t *testing.T) {
	for _, command := range []string{"init", "info", "run", "serve", "eval"} {
		t.Run(command, func(t *testing.T) {
			if err := run(
				[]string{command, "--help"},
				commandStreams{stdout: io.Discard, stderr: io.Discard},
			); err != nil {
				t.Fatalf("%s --help: %v", command, err)
			}
		})
	}
}

func TestInitCreatesAgentProject(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	var output strings.Builder
	if err := run(
		[]string{"init", root},
		commandStreams{stdout: &output, stderr: io.Discard},
	); err != nil {
		t.Fatal(err)
	}

	wantFiles := map[string]string{
		"agent/instructions.md": "# Identity\n\nYou are a helpful assistant.\n",
		"agent/agent.ts":        "export default { model: \"openai/gpt-5.4-mini\" };\n",
	}
	for name, want := range wantFiles {
		path := filepath.Join(root, name)
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(contents) != want {
			t.Fatalf("%s contents = %q", name, contents)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Fatalf("%s permissions = %o", name, info.Mode().Perm())
		}
	}
	if output.String() != "initialized "+root+"\n" {
		t.Fatalf("init output = %q", output.String())
	}
}

func TestInitPreflightsAllTargetCollisions(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		absent   string
	}{
		{
			name:     "instructions collision",
			existing: "agent/instructions.md",
			absent:   "agent/agent.ts",
		},
		{
			name:     "agent collision",
			existing: "agent/agent.ts",
			absent:   "agent/instructions.md",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			existingPath := filepath.Join(root, tt.existing)
			if err := os.MkdirAll(filepath.Dir(existingPath), 0o755); err != nil {
				t.Fatal(err)
			}
			const original = "keep this content\n"
			if err := os.WriteFile(existingPath, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}

			err := run(
				[]string{"init", root},
				commandStreams{stdout: io.Discard, stderr: io.Discard},
			)
			if err == nil || err.Error() != existingPath+" already exists" {
				t.Fatalf("init error = %v", err)
			}
			contents, readErr := os.ReadFile(existingPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(contents) != original {
				t.Fatalf("existing contents = %q", contents)
			}
			if _, statErr := os.Stat(filepath.Join(root, tt.absent)); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("absent target stat error = %v", statErr)
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
	options, err := parseServeOptions(nil, io.Discard)
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
	options, err := parseServeOptions(
		[]string{"--runtime", "eve", "--root", "/tmp/agent"},
		io.Discard,
	)
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
