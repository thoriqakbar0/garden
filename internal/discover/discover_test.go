package discover_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thoriqakbar0/garden/internal/discover"
)

func TestApplicationAtDiscoversFilesystemContract(t *testing.T) {
	root := t.TempDir()
	write(t, root, "agent/instructions.md", "Be useful.")
	write(t, root, "agent/agent.ts", `export default defineAgent({model: process.env.MODEL ?? "openai/gpt-5.4-mini"});`)
	write(t, root, "agent/tools/weather.ts", "")
	write(t, root, "agent/skills/research/SKILL.md", "")
	write(t, root, "agent/channels/slack.ts", "")
	write(t, root, "agent/subagents/reviewer/instructions.md", "")
	write(t, root, "agent/schedules/daily.ts", `defineSchedule({cron: "0 8 * * *"});`)
	write(t, root, "agent/schedules/hourly.mjs", `defineSchedule({cron: "0 * * * *"});`)
	write(t, root, "agent/schedules/reports/weekly.md", "---\ncron: \"0 9 * * 1\"\n---\nWrite the weekly report.\n")
	write(t, root, "evals/runtime/replay.eval.ts", "")

	app, err := discover.ApplicationAt(root)
	if err != nil {
		t.Fatal(err)
	}
	info := app.Info()
	if info.Model != "openai/gpt-5.4-mini" {
		t.Fatalf("model = %q", info.Model)
	}
	assertOne(t, info.Tools, "weather")
	assertOne(t, info.Skills, "research")
	assertOne(t, info.Channels, "slack")
	assertOne(t, info.Subagents, "reviewer")
	assertOne(t, info.Evals, "runtime/replay")
	if len(info.Schedules) != 3 ||
		info.Schedules[0].ID != "daily" || info.Schedules[0].Cron != "0 8 * * *" ||
		info.Schedules[1].ID != "hourly" || info.Schedules[1].Cron != "0 * * * *" ||
		info.Schedules[1].Path != "agent/schedules/hourly.mjs" ||
		info.Schedules[2].ID != "reports/weekly" || info.Schedules[2].Cron != "0 9 * * 1" ||
		info.Schedules[2].Path != "agent/schedules/reports/weekly.md" {
		t.Fatalf("schedules = %#v", info.Schedules)
	}
}

func TestApplicationAtRejectsDuplicateScheduleIDs(t *testing.T) {
	root := t.TempDir()
	write(t, root, "agent/instructions.md", "Be useful.")
	write(t, root, "agent/schedules/daily.ts", `defineSchedule({cron: "0 8 * * *"});`)
	write(t, root, "agent/schedules/daily.md", "---\ncron: \"0 9 * * *\"\n---\nDaily report.\n")

	_, err := discover.ApplicationAt(root)
	if err == nil || err.Error() != `duplicate schedule id "daily"` {
		t.Fatalf("error = %v", err)
	}
}

func write(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertOne(t *testing.T, got []string, want string) {
	t.Helper()
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %v, want [%s]", got, want)
	}
}
