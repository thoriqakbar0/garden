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
	info := app.Info()
	if info.Model != "anthropic/claude-sonnet-5" {
		t.Fatalf("model = %q", info.Model)
	}
	if len(info.Tools) != 1 || info.Tools[0] != "get_weather" {
		t.Fatalf("tools = %v", info.Tools)
	}
	if len(info.Skills) != 1 || info.Skills[0] != "get-weather" {
		t.Fatalf("skills = %v", info.Skills)
	}
	if info.Root != filepath.Clean(root) {
		t.Fatalf("root = %q, want %q", info.Root, filepath.Clean(root))
	}
}
