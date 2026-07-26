# Eve weather example

This is Garden's copy of the small weather-agent shape used by the official
Vercel eve repository. It proves that Garden discovers a normal Eve agent tree
and can run durable turns from the same project directory.

```sh
go build -o garden ./cmd/eve
./garden info --root examples/eve-weather
./garden run --root examples/eve-weather --message "What is the weather in Jakarta?"
./garden serve --root examples/eve-weather --addr 127.0.0.1:38181
```

The current Go runtime returns its deterministic workflow response
(`stress-ack:<turn>:<message>`). It discovers the authored TypeScript weather
tool but does not execute TypeScript or call a model yet.

Based on `apps/fixtures/weather-agent` from `vercel/eve`, Apache-2.0, baseline
commit `05f348023d4268c974c225c1189a283ace20b742`.
