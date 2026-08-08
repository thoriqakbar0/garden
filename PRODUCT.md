# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

React landing page using Vite and TanStack Router in `website/`, selected by the user. UI work standardizes on Tailwind CSS: migrate touched styling to utilities and reserve custom CSS for effects or behavior that utilities cannot express clearly. Garden itself remains a Go CLI.

## Users

Primary audience (inferred from the repository and requested landing page): developers evaluating or operating Eve agents who want a local, self-hosted execution path without adopting a hosted Garden service.

## Product purpose

Garden is an independent, self-hosted Go implementation of a documented Eve-compatible subset. Eve by Vercel remains the complete framework and runtime. Garden may validate and supervise a pinned project-local Eve process for compatibility, but that path is still Eve—not a second Garden runtime. Success means a developer can choose Eve or Garden without misleading compatibility claims.

## Positioning

Garden is a self-hosted Go alternative rather than a hosted agent platform. It runs the smaller documented Eve-compatible contract as one process and can optionally supervise the exact project-local Eve package without claiming ownership of Eve’s behavior.

## Operating context

Garden is installed from source with `make install`, used from a terminal, and pointed at an Eve-shaped project containing `agent/`. Garden requires Go 1.25+ plus Codex or credentials for a native OpenAI, Anthropic, Google, or OpenAI-compatible provider. Supervising Eve with `--runtime eve` instead requires Node 24 and project-local `eve@0.27.6`.

## Capabilities and constraints

- When Garden launches Eve, Eve owns authored semantics and runtime behavior.
- Garden supports sessions, streaming, tool/model turns, cancellation, and local recovery, but not arbitrary authored TypeScript.
- Native OpenAI, Anthropic, and Google model providers support the shared model and native-tool loop; OpenAI-compatible endpoints remain supported.
- Provider responses normalize provider/model identity, stop reason, and token/cache usage into durable Eve step metadata.
- Cloudflare model endpoints and AI Gateway are supported model connections.
- Cloudflare deployment is not implemented and must not be represented as available.
- Distributed storage is not implemented.
- Compatibility claims must match `COMPATIBILITY.md` and `UPSTREAM.md`.

## Brand commitments

The product name is Garden. The user selected a “Cultivated systems” direction: technical and restrained, with botanical structure rather than decorative garden clichés.

## Evidence on hand

- `README.md`, `COMPATIBILITY.md`, `TESTING.md`, and `UPSTREAM.md`.
- Working CLI examples in `examples/eve-weather/` and `examples/eve-parity/`.
- No testimonials, customer logos, production benchmarks, pricing, or hosted-service claims are available; do not fabricate them.

## Product principles

- Preserve authored Eve semantics whenever Eve owns execution.
- Be explicit about the boundary between Eve by Vercel and Garden’s compatible Go subset.
- Keep operation local and user-owned by default.
- Prove compatibility through executable tests rather than broad claims.
- Prefer a small, legible command surface.

## Accessibility & inclusion

The marketing page must use semantic HTML, complete keyboard access, visible focus, reduced-motion support, sufficient contrast, and responsive reflow at 320 px.
