package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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
	"github.com/thoriqakbar0/garden/internal/server"
	"github.com/thoriqakbar0/garden/internal/workflow"
)

const version = "0.1.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "eve:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "init":
		return initProject(args[1:])
	case "info":
		return info(args[1:])
	case "run":
		return runOnce(args[1:])
	case "serve", "dev", "start":
		return serve(args[1:])
	case "eval":
		return eval(args[1:])
	case "version", "--version", "-v":
		fmt.Println(version)
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%w", args[0], usage())
	}
}

func usage() error {
	fmt.Fprintln(os.Stderr, `garden: a self-hosted runtime for Eve agents

Usage:
  eve init [directory]
  eve info [--root directory]
  eve run [--root directory] --message text [--session id]
  eve serve [--root directory] [--addr 127.0.0.1:3000]
  eve eval [--root directory] --list
  eve version`)
	return errors.New("a command is required")
}

func initProject(args []string) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil {
		return err
	}
	root := "."
	if flags.NArg() == 1 {
		root = flags.Arg(0)
	} else if flags.NArg() > 1 {
		return errors.New("init accepts at most one directory")
	}
	files := map[string]string{
		"agent/instructions.md": "# Identity\n\nYou are a helpful assistant.\n",
		"agent/agent.ts":        "export default { model: \"openai/gpt-5.4-mini\" };\n",
	}
	for name, contents := range files {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			return err
		}
	}
	fmt.Println("initialized", root)
	return nil
}

func info(args []string) error {
	root, _, err := commonFlags("info", args, false)
	if err != nil {
		return err
	}
	app, err := discover.ApplicationAt(root)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(app)
}

func runOnce(args []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
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
	runner, err := agent.RunnerFromEnvironment(app)
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
	return json.NewEncoder(os.Stdout).Encode(result)
}

func serve(args []string) (returnErr error) {
	root, addr, err := commonFlags("serve", args, true)
	if err != nil {
		return err
	}
	app, err := discover.ApplicationAt(root)
	if err != nil {
		return err
	}
	runner, err := agent.RunnerFromEnvironment(app)
	if err != nil {
		return err
	}
	store, err := workflow.OpenRunner(filepath.Join(root, ".eve", "workflow-data"), runner)
	if err != nil {
		return err
	}
	defer func() { returnErr = errors.Join(returnErr, store.Close()) }()
	handler, err := authenticatedHandler(addr, os.Getenv("GARDEN_AUTH_TOKEN"), server.Handler(app, store))
	if err != nil {
		return err
	}
	log.Printf("garden listening on %s", addr)
	httpServer := &http.Server{
		Addr:              addr,
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

func eval(args []string) error {
	flags := flag.NewFlagSet("eval", flag.ContinueOnError)
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
	fmt.Println(strings.Join(app.Evals, "\n"))
	return nil
}

func commonFlags(name string, args []string, withAddr bool) (string, string, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	root := flags.String("root", ".", "project root")
	addr := flags.String("addr", "127.0.0.1:3000", "listen address")
	if !withAddr {
		flags.SetOutput(os.Stderr)
	}
	if err := flags.Parse(args); err != nil {
		return "", "", err
	}
	return *root, *addr, nil
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
