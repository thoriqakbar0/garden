package evehost

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHostRunsPinnedProjectLocalEve(t *testing.T) {
	root := createEveProject(t, pinnedEveVersion, `#!/bin/sh
printf '%s\n' "$@"
`)
	host, err := Open(root, "127.0.0.1:4312")
	if err != nil {
		t.Fatal(err)
	}
	var output safeBuffer
	if err := host.Run(context.Background(), &output, &output); err != nil {
		t.Fatal(err)
	}
	want := "dev\n--no-ui\n--host\n127.0.0.1\n--port\n4312\n"
	if output.String() != want {
		t.Fatalf("arguments = %q, want %q", output.String(), want)
	}
}

func TestHostRejectsUnpinnedEve(t *testing.T) {
	root := createEveProject(t, "9.9.9", "#!/bin/sh\nexit 0\n")
	_, err := Open(root, "127.0.0.1:4312")
	if err == nil || !strings.Contains(err.Error(), "requires project-local eve@"+pinnedEveVersion) {
		t.Fatalf("version error = %v", err)
	}
}

func TestHostRequiresProjectLocalEve(t *testing.T) {
	_, err := Open(t.TempDir(), "127.0.0.1:4312")
	if err == nil || !strings.Contains(err.Error(), "requires project-local eve@"+pinnedEveVersion) {
		t.Fatalf("missing install error = %v", err)
	}
}

func TestHostRejectsCLIOutsideNodeModules(t *testing.T) {
	root := createEveProject(t, pinnedEveVersion, "#!/bin/sh\nexit 0\n")
	externalCommand := filepath.Join(t.TempDir(), "eve")
	if err := os.WriteFile(externalCommand, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	command := filepath.Join(root, "node_modules", ".bin", "eve")
	if err := os.Remove(command); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalCommand, command); err != nil {
		t.Fatal(err)
	}

	_, err := Open(root, "127.0.0.1:4312")
	if err == nil || !strings.Contains(err.Error(), "resolves outside node_modules") {
		t.Fatalf("escaped CLI error = %v", err)
	}
}

func TestHostRejectsNonExecutableCLI(t *testing.T) {
	root := createEveProject(t, pinnedEveVersion, "plain text\n")
	command := filepath.Join(root, "node_modules", "eve", "bin", "eve")
	if err := os.Chmod(command, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Open(root, "127.0.0.1:4312")
	if err == nil || !strings.Contains(err.Error(), "is not executable") {
		t.Fatalf("non-executable CLI error = %v", err)
	}
}

func TestHostRejectsInvalidAddress(t *testing.T) {
	root := createEveProject(t, pinnedEveVersion, "#!/bin/sh\nexit 0\n")
	_, err := Open(root, ":4312")
	if err == nil || !strings.Contains(err.Error(), "must include a host and port") {
		t.Fatalf("address error = %v", err)
	}
}

func TestHostDoesNotStartWithCancelledContext(t *testing.T) {
	root := createEveProject(t, pinnedEveVersion, `#!/bin/sh
printf 'unexpected-start\n'
`)
	host, err := Open(root, "127.0.0.1:4312")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output safeBuffer
	err = host.Run(ctx, &output, &output)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	if output.String() != "" {
		t.Fatalf("cancelled host output = %q", output.String())
	}
}

func TestHostCancellationStopsEveProcess(t *testing.T) {
	root := createEveProject(t, pinnedEveVersion, `#!/bin/sh
printf 'started\n'
exec sleep 30
`)
	host, err := Open(root, "127.0.0.1:4312")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var output safeBuffer
	done := make(chan error, 1)
	go func() { done <- host.Run(ctx, &output, &output) }()
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(output.String(), "started") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Eve process did not stop after cancellation")
	}
}

type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *safeBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(data)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func createEveProject(t *testing.T, version, script string) string {
	t.Helper()
	root := t.TempDir()
	packageRoot := filepath.Join(root, "node_modules", "eve")
	binRoot := filepath.Join(packageRoot, "bin")
	if err := os.MkdirAll(binRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"eve","version":"` + version + `"}`
	if err := os.WriteFile(filepath.Join(packageRoot, "package.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	command := filepath.Join(binRoot, "eve")
	if err := os.WriteFile(command, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	localBin := filepath.Join(root, "node_modules", ".bin")
	if err := os.MkdirAll(localBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "eve", "bin", "eve"), filepath.Join(localBin, "eve")); err != nil {
		t.Fatal(err)
	}
	return root
}
