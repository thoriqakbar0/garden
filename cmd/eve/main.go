package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/thoriqakbar0/garden/internal/discover"
	"github.com/thoriqakbar0/garden/internal/server"
	"github.com/thoriqakbar0/garden/internal/vercel"
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
	case "build":
		return build(args[1:])
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
	fmt.Fprintln(os.Stderr, `garden: a single-binary eve runtime

Usage:
  eve init [directory]
  eve info [--root directory]
  eve run [--root directory] --message text [--session id]
  eve serve [--root directory] [--addr :3000]
  eve build [--root directory]
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
	if _, err := discover.ApplicationAt(*root); err != nil {
		return err
	}
	store, err := workflow.Open(filepath.Join(*root, ".eve", "workflow-data"), workflow.EchoResponder)
	if err != nil {
		return err
	}
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

func serve(args []string) error {
	root, addr, err := commonFlags("serve", args, true)
	if err != nil {
		return err
	}
	app, err := discover.ApplicationAt(root)
	if err != nil {
		return err
	}
	store, err := workflow.Open(filepath.Join(root, ".eve", "workflow-data"), workflow.EchoResponder)
	if err != nil {
		return err
	}
	log.Printf("eve listening on %s", addr)
	return http.ListenAndServe(addr, server.Handler(app, store))
}

func build(args []string) error {
	root, _, err := commonFlags("build", args, false)
	if err != nil {
		return err
	}
	app, err := discover.ApplicationAt(root)
	if err != nil {
		return err
	}
	path, err := vercel.WriteConfig(root, app)
	if err != nil {
		return err
	}
	fmt.Println(path)
	return nil
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
	addr := flags.String("addr", ":3000", "listen address")
	if !withAddr {
		flags.SetOutput(os.Stderr)
	}
	if err := flags.Parse(args); err != nil {
		return "", "", err
	}
	return *root, *addr, nil
}
