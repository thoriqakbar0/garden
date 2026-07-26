// Package discover maps an eve-style filesystem tree into inert application data.
package discover

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	cronPattern  = regexp.MustCompile(`cron\s*:\s*["']([^"']+)["']`)
	modelPattern = regexp.MustCompile(`(?:\?\?\s*)?["']([a-z0-9_-]+/[a-zA-Z0-9._:-]+)["']`)
)

// Application is the discovered, serializable description of one authored agent.
type Application struct {
	Root         string     `json:"root"`
	Instructions string     `json:"instructions"`
	Model        string     `json:"model,omitempty"`
	Tools        []string   `json:"tools"`
	Skills       []string   `json:"skills"`
	Channels     []string   `json:"channels"`
	Connections  []string   `json:"connections"`
	Subagents    []string   `json:"subagents"`
	Schedules    []Schedule `json:"schedules"`
	Evals        []string   `json:"evals"`
}

// Schedule describes a schedule whose stable identifier is derived from its file path.
type Schedule struct {
	ID   string `json:"id"`
	Cron string `json:"cron,omitempty"`
	Path string `json:"path"`
}

// ApplicationAt discovers an eve-compatible agent rooted at projectRoot.
func ApplicationAt(projectRoot string) (Application, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return Application{}, fmt.Errorf("resolve project root: %w", err)
	}
	agentRoot := filepath.Join(root, "agent")
	instructionsPath := filepath.Join(agentRoot, "instructions.md")
	instructions, err := os.ReadFile(instructionsPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Application{}, fmt.Errorf("discover agent: %s is required", instructionsPath)
		}
		return Application{}, fmt.Errorf("read instructions: %w", err)
	}

	app := Application{Root: root, Instructions: string(instructions)}
	app.Tools, err = namedFiles(filepath.Join(agentRoot, "tools"), sourceFile)
	if err != nil {
		return Application{}, err
	}
	app.Skills, err = namedSkills(filepath.Join(agentRoot, "skills"))
	if err != nil {
		return Application{}, err
	}
	app.Channels, err = namedFiles(filepath.Join(agentRoot, "channels"), sourceFile)
	if err != nil {
		return Application{}, err
	}
	app.Connections, err = namedFiles(filepath.Join(agentRoot, "connections"), sourceFile)
	if err != nil {
		return Application{}, err
	}
	app.Subagents, err = namedDirectories(filepath.Join(agentRoot, "subagents"))
	if err != nil {
		return Application{}, err
	}
	app.Schedules, err = schedules(filepath.Join(agentRoot, "schedules"), root)
	if err != nil {
		return Application{}, err
	}
	app.Evals, err = namedFiles(filepath.Join(root, "evals"), func(path string) bool {
		return strings.HasSuffix(path, ".eval.ts") || strings.HasSuffix(path, ".eval.js")
	})
	if err != nil {
		return Application{}, err
	}
	agentSource, readErr := os.ReadFile(filepath.Join(agentRoot, "agent.ts"))
	if readErr == nil {
		if match := modelPattern.FindSubmatch(agentSource); len(match) == 2 {
			app.Model = string(match[1])
		}
	} else if !errors.Is(readErr, fs.ErrNotExist) {
		return Application{}, fmt.Errorf("read agent definition: %w", readErr)
	}
	return app, nil
}

func sourceFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".ts" || ext == ".js" || ext == ".mjs"
}

func namedFiles(root string, accept func(string) bool) ([]string, error) {
	var names []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return fs.SkipDir
		}
		if err != nil {
			return err
		}
		if entry.IsDir() || !accept(path) {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(filepath.ToSlash(relative), filepath.Ext(relative))
		name = strings.TrimSuffix(name, ".eval")
		names = append(names, name)
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", root, err)
	}
	slices.Sort(names)
	if names == nil {
		return []string{}, nil
	}
	return names, nil
}

func namedSkills(root string) ([]string, error) {
	files, err := namedFiles(root, func(path string) bool {
		return strings.EqualFold(filepath.Base(path), "SKILL.md") || filepath.Ext(path) == ".md"
	})
	if err != nil {
		return nil, err
	}
	for i := range files {
		files[i] = strings.TrimSuffix(files[i], "/SKILL")
	}
	return slices.Compact(files), nil
}

func namedDirectories(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", root, err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)
	return names, nil
}

func schedules(root, projectRoot string) ([]Schedule, error) {
	files, err := namedFiles(root, sourceFile)
	if err != nil {
		return nil, err
	}
	result := make([]Schedule, 0, len(files))
	for _, id := range files {
		path := filepath.Join(root, filepath.FromSlash(id)+".ts")
		source, readErr := os.ReadFile(path)
		if errors.Is(readErr, fs.ErrNotExist) {
			path = filepath.Join(root, filepath.FromSlash(id)+".js")
			source, readErr = os.ReadFile(path)
		}
		if readErr != nil {
			return nil, fmt.Errorf("read schedule %s: %w", id, readErr)
		}
		schedule := Schedule{ID: id}
		if relative, relativeErr := filepath.Rel(projectRoot, path); relativeErr == nil {
			schedule.Path = filepath.ToSlash(relative)
		}
		if match := cronPattern.FindSubmatch(source); len(match) == 2 {
			schedule.Cron = string(match[1])
		}
		result = append(result, schedule)
	}
	return result, nil
}
