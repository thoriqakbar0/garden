# Garden website

The public landing page for Garden. It is a standalone Next.js application; the
Garden CLI remains the Go project at the repository root.

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
