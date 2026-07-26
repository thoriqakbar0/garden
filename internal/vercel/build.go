// Package vercel emits Vercel configuration from a discovered agent.
package vercel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/thoriqakbar0/garden/internal/discover"
)

// WriteConfig emits routes and cron declarations for Vercel's native Go runtime.
func WriteConfig(projectRoot string, app discover.Application) (string, error) {
	type rewrite struct {
		Source      string `json:"source"`
		Destination string `json:"destination"`
	}
	type cron struct {
		Path     string `json:"path"`
		Schedule string `json:"schedule"`
	}
	config := struct {
		Schema   string    `json:"$schema"`
		Rewrites []rewrite `json:"rewrites"`
		Crons    []cron    `json:"crons,omitempty"`
	}{
		Schema: "https://openapi.vercel.sh/vercel.json",
		Rewrites: []rewrite{{
			Source:      "/eve/v1/:path*",
			Destination: "/api/eve?path=:path*",
		}},
	}
	for _, schedule := range app.Schedules {
		if schedule.Cron != "" {
			config.Crons = append(config.Crons, cron{
				Path:     "/eve/v1/schedules/" + schedule.ID + "/dispatch",
				Schedule: schedule.Cron,
			})
		}
	}
	path := filepath.Join(projectRoot, "vercel.json")
	file, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create Vercel config: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		file.Close()
		return "", fmt.Errorf("write Vercel config: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close Vercel config: %w", err)
	}
	return path, nil
}
