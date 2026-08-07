# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

React landing page using Vite and TanStack Router in `website/`, selected by the user. Garden itself remains a Go CLI.

## Users

Primary audience (inferred from the repository and requested landing page): developers evaluating or operating Eve agents who want a local, self-hosted execution path without adopting a hosted Garden service.

## Product purpose

Garden runs Eve-shaped agents locally in two explicit execution modes: the pinned official Eve runtime for authored TypeScript compatibility, and a standalone native Go runtime for a smaller conversation contract. Success means a developer can understand the distinction, install Garden, and run the appropriate mode without misleading compatibility claims.

## Positioning

Garden is a local runtime bridge rather than a hosted agent platform: it can supervise the exact project-local Eve runtime or run a smaller Eve-shaped contract as one Go process.

## Operating context

Garden is installed from source with `make install`, used from a terminal, and pointed at an Eve-shaped project containing `agent/`. Official mode requires Node 24 and project-local `eve@0.27.6`; native mode requires Go 1.25+ plus Codex or an OpenAI-compatible endpoint.

## Capabilities and constraints

- Official mode delegates authored semantics to exact project-local Eve.
- Native mode supports sessions, streaming, tool/model turns, cancellation, and local recovery, but not arbitrary authored TypeScript.
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

- Preserve authored Eve semantics when official Eve owns execution.
- Be explicit about native versus official capability.
- Keep operation local and user-owned by default.
- Prove compatibility through executable tests rather than broad claims.
- Prefer a small, legible command surface.

## Accessibility & inclusion

The marketing page must use semantic HTML, complete keyboard access, visible focus, reduced-motion support, sufficient contrast, and responsive reflow at 320 px.
