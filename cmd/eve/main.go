package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/thoriqakbar0/garden/internal/agent"
	"github.com/thoriqakbar0/garden/internal/discover"
	"github.com/thoriqakbar0/garden/internal/evehost"
	"github.com/thoriqakbar0/garden/internal/server"
	"github.com/thoriqakbar0/garden/internal/workflow"
)

const version = "0.1.0"

const helpText = `garden: run Eve agents locally

Usage:
  garden <command> [options]

Commands:
  init [directory]             Create an Eve-shaped agent project.
  info [--root directory]      Inspect a discovered agent project.
  run --message text           Run one native agent turn.
  serve [options]              Start the native or official Eve server.
  eval --list                  List discovered evaluations.
  help                         Show this help.
  version                      Show the Garden version.

Install from a source checkout:
  make install

The default destination is $HOME/.local/bin/garden.
Set BINDIR to select another destination:
  make install BINDIR="$HOME/bin"

Verify the installation:
  command -v garden
  garden version

Run "garden <command> --help" to show command options.`

func main() {
	streams := commandStreams{stdout: os.Stdout, stderr: os.Stderr}
	if err := run(os.Args[1:], streams); err != nil {
		fmt.Fprintln(streams.stderr, "garden:", err)
		os.Exit(1)
	}
}

type commandStreams struct {
	stdout io.Writer
	stderr io.Writer
}

func run(args []string, streams commandStreams) error {
	if len(args) == 0 {
		return usageError("a command is required")
	}
	var commandErr error
	switch args[0] {
	case "init":
		commandErr = initProject(args[1:], streams)
	case "info":
		commandErr = info(args[1:], streams)
	case "run":
		commandErr = runOnce(args[1:], streams)
	case "serve", "dev", "start":
		commandErr = serve(args[1:], streams)
	case "eval":
		commandErr = eval(args[1:], streams)
	case "help", "--help", "-h":
		if _, err := fmt.Fprintln(streams.stdout, helpText); err != nil {
			return fmt.Errorf("write help: %w", err)
		}
		return nil
	case "version", "--version", "-v":
		if _, err := fmt.Fprintln(streams.stdout, version); err != nil {
			return fmt.Errorf("write version: %w", err)
		}
		return nil
	default:
		return usageError(fmt.Sprintf("unknown command %q", args[0]))
	}
	if errors.Is(commandErr, flag.ErrHelp) {
		return nil
	}
	return commandErr
}

func usageError(message string) error {
	return fmt.Errorf("%s\n\n%s", message, helpText)
}

func initProject(args []string, streams commandStreams) error {
	flags := flag.NewFlagSet("garden init", flag.ContinueOnError)
	flags.SetOutput(streams.stderr)
	if err := flags.Parse(args); err != nil {
		return err
	}
	root := "."
	if flags.NArg() == 1 {
		root = flags.Arg(0)
	} else if flags.NArg() > 1 {
		return errors.New("init accepts at most one directory")
	}
	files := []struct {
		name     string
		contents string
	}{
		{name: "agent/instructions.md", contents: "# Identity\n\nYou are a helpful assistant.\n"},
		{name: "agent/agent.ts", contents: "export default { model: \"openai/gpt-5.4-mini\" };\n"},
	}
	for _, file := range files {
		path := filepath.Join(root, file.name)
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("%s already exists", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	for _, file := range files {
		path := filepath.Join(root, file.name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(file.contents), 0o644); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(streams.stdout, "initialized", root)
	if err != nil {
		return fmt.Errorf("write init result: %w", err)
	}
	return nil
}

func info(args []string, streams commandStreams) error {
	flags := flag.NewFlagSet("garden info", flag.ContinueOnError)
	flags.SetOutput(streams.stderr)
	root := flags.String("root", ".", "project root")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("info does not accept positional arguments")
	}
	app, err := discover.ApplicationAt(*root)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(streams.stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(app.Info())
}

func runOnce(args []string, streams commandStreams) error {
	flags := flag.NewFlagSet("garden run", flag.ContinueOnError)
	flags.SetOutput(streams.stderr)
	root := flags.String("root", ".", "project root")
	message := flags.String("message", "", "message to send")
	sessionID := flags.String("session", "", "existing session id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *message == "" {
		return errors.New("run requires a non-empty --message")
	}
	app, err := discover.ApplicationAt(*root)
	if err != nil {
		return err
	}
	runner, err := agent.RunnerFromEnvironment(app.Native())
	if err != nil {
		return err
	}
	store, err := workflow.OpenRunner(filepath.Join(*root, ".eve", "workflow-data"), runner)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	id := *sessionID
	if id == "" {
		id, err = store.CreateSession()
		if err != nil {
			return err
		}
	}
	result, err := store.Send(context.Background(), id, *message)
	if err != nil {
		return err
	}
	return json.NewEncoder(streams.stdout).Encode(result)
}

func serve(args []string, streams commandStreams) (returnErr error) {
	options, err := parseServeOptions(args, streams.stderr)
	if err != nil {
		return err
	}
	if options.runtime == "eve" {
		return serveOfficialEve(options, streams)
	}
	app, err := discover.ApplicationAt(options.root)
	if err != nil {
		return err
	}
	runner, err := agent.RunnerFromEnvironment(app.Native())
	if err != nil {
		return err
	}
	store, err := workflow.OpenRunner(filepath.Join(options.root, ".eve", "workflow-data"), runner)
	if err != nil {
		return err
	}
	defer func() { returnErr = errors.Join(returnErr, store.Close()) }()
	handler, err := authenticatedHandler(options.addr, os.Getenv("GARDEN_AUTH_TOKEN"), server.Handler(app.Manifest(), store))
	if err != nil {
		return err
	}
	log.New(streams.stderr, "", log.LstdFlags).Printf("garden listening on %s", options.addr)
	httpServer := &http.Server{
		Addr:              options.addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-stop:
			_ = httpServer.Close()
		case <-done:
		}
	}()
	err = httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

type serveOptions struct {
	addr    string
	root    string
	runtime string
}

func parseServeOptions(args []string, stderr io.Writer) (serveOptions, error) {
	flags := flag.NewFlagSet("garden serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "project root")
	addr := flags.String("addr", "127.0.0.1:3000", "listen address")
	runtimeName := flags.String("runtime", "native", "runtime implementation: native or eve")
	if err := flags.Parse(args); err != nil {
		return serveOptions{}, err
	}
	if flags.NArg() != 0 {
		return serveOptions{}, errors.New("serve does not accept positional arguments")
	}
	runtimeValue := strings.ToLower(strings.TrimSpace(*runtimeName))
	if runtimeValue != "native" && runtimeValue != "eve" {
		return serveOptions{}, errors.New("serve --runtime must be native or eve")
	}
	return serveOptions{addr: *addr, root: *root, runtime: runtimeValue}, nil
}

func serveOfficialEve(options serveOptions, streams commandStreams) error {
	host, err := evehost.Open(options.root, options.addr)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err = host.Run(ctx, streams.stdout, streams.stderr)
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		return nil
	}
	return err
}

func eval(args []string, streams commandStreams) error {
	flags := flag.NewFlagSet("garden eval", flag.ContinueOnError)
	flags.SetOutput(streams.stderr)
	root := flags.String("root", ".", "project root")
	list := flags.Bool("list", false, "list discovered evals")
	if err := flags.Parse(args); err != nil {
		return err
	}
	app, err := discover.ApplicationAt(*root)
	if err != nil {
		return err
	}
	if !*list {
		return errors.New("TypeScript eval execution is not ported; use --list or write native Go compatibility tests")
	}
	_, err = fmt.Fprintln(streams.stdout, strings.Join(app.Info().Evals, "\n"))
	if err != nil {
		return fmt.Errorf("write eval list: %w", err)
	}
	return nil
}

func authenticatedHandler(addr, configuredToken string, next http.Handler) (http.Handler, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid listen address %q: %w", addr, err)
	}
	token := strings.TrimSpace(configuredToken)
	if token == "" {
		if loopbackHost(host) {
			return next, nil
		}
		return nil, errors.New("non-loopback serving requires GARDEN_AUTH_TOKEN")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		authorization := r.Header.Get("Authorization")
		provided := ""
		if strings.HasPrefix(authorization, prefix) {
			provided = strings.TrimPrefix(authorization, prefix)
		}
		if len(provided) != len(token) || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}), nil
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
