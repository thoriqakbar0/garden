# Official Eve parity fixture

This credential-free fixture proves that Garden can supervise the pinned official
`eve@0.27.6` runtime while Eve itself compiles authored TypeScript, runs a
sandboxed Bash command, executes an authored tool, feeds both results back to a
deterministic authored model, and completes the public event stream.

From the repository root:

```sh
make test-official
```

The fixture uses Eve's local `mockModel` and `just-bash` sandbox, so it requires
Node 24+ and an npm install but no model-provider credential or Docker daemon.
