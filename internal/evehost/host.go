// Package evehost supervises the official Eve runtime for exact authored-agent execution.
package evehost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	pinnedEveVersion = "0.27.6"
	maxPackageBytes  = 1 << 20
	shutdownGrace    = 5 * time.Second
)

// Host runs the project-local official Eve CLI at Garden's pinned version.
type Host struct {
	command string
	host    string
	port    string
	root    string
}

type packageManifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Open validates a project-local official Eve installation and listen address.
func Open(root, address string) (*Host, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, errors.New("resolve Eve project root")
	}
	info, err := os.Stat(absoluteRoot)
	if err != nil {
		return nil, fmt.Errorf("open Eve project root: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("Eve project root must be a directory")
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil || strings.TrimSpace(host) == "" {
		return nil, errors.New("Eve listen address must include a host and port")
	}
	parsedPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return nil, errors.New("Eve listen port must be between 0 and 65535")
	}
	port = strconv.FormatUint(parsedPort, 10)

	packageRoot := filepath.Join(absoluteRoot, "node_modules", "eve")
	manifest, err := readPackageManifest(filepath.Join(packageRoot, "package.json"))
	if err != nil {
		return nil, err
	}
	if manifest.Name != "eve" || manifest.Version != pinnedEveVersion {
		return nil, fmt.Errorf(
			"Garden parity mode requires project-local eve@%s; found %s@%s",
			pinnedEveVersion, manifest.Name, manifest.Version,
		)
	}

	command := filepath.Join(absoluteRoot, "node_modules", ".bin", "eve")
	resolvedCommand, err := filepath.EvalSymlinks(command)
	if err != nil {
		return nil, fmt.Errorf("resolve project-local Eve CLI: %w", err)
	}
	resolvedNodeModules, err := filepath.EvalSymlinks(filepath.Join(absoluteRoot, "node_modules"))
	if err != nil {
		return nil, errors.New("resolve project-local node_modules")
	}
	if !within(resolvedNodeModules, resolvedCommand) {
		return nil, errors.New("project-local Eve CLI resolves outside node_modules")
	}
	commandInfo, err := os.Stat(resolvedCommand)
	if err != nil {
		return nil, fmt.Errorf("open project-local Eve CLI: %w", err)
	}
	if commandInfo.IsDir() || commandInfo.Mode().Perm()&0o111 == 0 {
		return nil, errors.New("project-local Eve CLI is not executable")
	}

	return &Host{command: resolvedCommand, host: host, port: port, root: absoluteRoot}, nil
}

func readPackageManifest(path string) (packageManifest, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return packageManifest{}, fmt.Errorf(
				"Garden parity mode requires project-local eve@%s; run the project's package-manager install",
				pinnedEveVersion,
			)
		}
		return packageManifest{}, errors.New("open project-local Eve package manifest")
	}
	defer func() { _ = file.Close() }()

	limited := io.LimitReader(file, maxPackageBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return packageManifest{}, errors.New("read project-local Eve package manifest")
	}
	if len(data) > maxPackageBytes {
		return packageManifest{}, errors.New("project-local Eve package manifest exceeds 1 MiB")
	}
	var manifest packageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return packageManifest{}, errors.New("project-local Eve package manifest is malformed")
	}
	return manifest, nil
}

func within(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// Run starts Eve's headless development host and owns its complete process lifetime.
func (h *Host) Run(ctx context.Context, stdout, stderr io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	command := exec.Command(h.command, "dev", "--no-ui", "--host", h.host, "--port", h.port)
	command.Dir = h.root
	command.Env = os.Environ()
	command.Stdout = stdout
	command.Stderr = stderr
	configureProcess(command)
	if err := command.Start(); err != nil {
		return errors.New("start project-local Eve runtime")
	}

	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			return nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("project-local Eve runtime exited with status %d", exitErr.ExitCode())
		}
		return errors.New("project-local Eve runtime failed")
	case <-ctx.Done():
		terminateProcess(command)
		select {
		case <-done:
		case <-time.After(shutdownGrace):
			killProcess(command)
			<-done
		}
		return ctx.Err()
	}
}
