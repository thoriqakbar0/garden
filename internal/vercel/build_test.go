package vercel_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/thoriqakbar0/garden/internal/discover"
	"github.com/thoriqakbar0/garden/internal/vercel"
)

func TestWriteConfigIncludesRoutesAndCron(t *testing.T) {
	root := t.TempDir()
	path, err := vercel.WriteConfig(root, discover.Application{
		Schedules: []discover.Schedule{{ID: "heartbeat", Cron: "0 8 * * *"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Crons []struct {
			Path string `json:"path"`
		} `json:"crons"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Crons) != 1 || config.Crons[0].Path != "/eve/v1/schedules/heartbeat/dispatch" {
		t.Fatalf("config = %s", data)
	}
}
