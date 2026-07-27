# Eve weather example

This is Garden's copy of the small weather-agent shape used by the official
Vercel eve repository. It proves that Garden discovers a normal Eve agent tree
and runs a real model → native `get_weather` tool → model turn.

```sh
go build -o garden ./cmd/eve
export GARDEN_MODEL_BACKEND=openai
export GARDEN_OPENAI_API_KEY=...
export GARDEN_MODEL=gpt-5.4
./garden info --root examples/eve-weather
./garden run --root examples/eve-weather --message "What is the weather in Jakarta?"
./garden serve --root examples/eve-weather --addr 127.0.0.1:38181
```

Garden does not execute the TypeScript source. The discovered `get_weather`
identifier binds to a deterministic native Go implementation that returns the
same fixture shape. Normal execution requires an explicitly configured model
backend and never falls back to an echo.

Based on `apps/fixtures/weather-agent` from `vercel/eve`, Apache-2.0, baseline
commit `05f348023d4268c974c225c1189a283ace20b742`.
