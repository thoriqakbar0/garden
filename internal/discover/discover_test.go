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
	if app.Model != "openai/gpt-5.4-mini" {
		t.Fatalf("model = %q", app.Model)
	}
	assertOne(t, app.Tools, "weather")
	assertOne(t, app.Skills, "research")
	assertOne(t, app.Channels, "slack")
	assertOne(t, app.Subagents, "reviewer")
	assertOne(t, app.Evals, "runtime/replay")
	if len(app.Schedules) != 3 ||
		app.Schedules[0].ID != "daily" || app.Schedules[0].Cron != "0 8 * * *" ||
		app.Schedules[1].ID != "hourly" || app.Schedules[1].Cron != "0 * * * *" ||
		app.Schedules[1].Path != "agent/schedules/hourly.mjs" ||
		app.Schedules[2].ID != "reports/weekly" || app.Schedules[2].Cron != "0 9 * * 1" ||
		app.Schedules[2].Path != "agent/schedules/reports/weekly.md" {
		t.Fatalf("schedules = %#v", app.Schedules)
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
