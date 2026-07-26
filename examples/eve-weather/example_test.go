package example_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thoriqakbar0/garden/internal/discover"
)

func TestOfficialEveShapeIsRunnableByGarden(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	app, err := discover.ApplicationAt(root)
	if err != nil {
		t.Fatal(err)
	}
	if app.Model != "anthropic/claude-sonnet-5" {
		t.Fatalf("model = %q", app.Model)
	}
	if len(app.Tools) != 1 || app.Tools[0] != "get_weather" {
		t.Fatalf("tools = %v", app.Tools)
	}
	if len(app.Skills) != 1 || app.Skills[0] != "get-weather" {
		t.Fatalf("skills = %v", app.Skills)
	}
	if app.Root != filepath.Clean(root) {
		t.Fatalf("root = %q, want %q", app.Root, filepath.Clean(root))
	}
}
