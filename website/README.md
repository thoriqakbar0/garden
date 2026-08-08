# Garden website

The public landing page for Garden is a standalone React application built with Vite and TanStack Router; the Garden CLI remains the Go project at the repository root. Tailwind CSS is the required default for new or changed UI styling: migrate touched styles to utilities and keep custom CSS only for effects or behavior Tailwind cannot express clearly.

## Develop

```sh
npm ci
npm run dev
```

Open <http://localhost:3000>.

## Verify

```sh
npm run lint
npm run build
```

Product claims on the page must remain consistent with the repository’s
`README.md`, `COMPATIBILITY.md`, `TESTING.md`, and `UPSTREAM.md`. Cloudflare
model connectivity is supported, but Cloudflare deployment is not currently a
shipping capability.
