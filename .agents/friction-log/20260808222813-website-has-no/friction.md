---
title: 'Website has no production bootstrap smoke test'
severity: 'major'
---

## Expected Behavior

CI verifies that the built website renders its hero, and that an actionable fallback remains visible when the entry module cannot load.

## Current Behavior

The website package has lint and build scripts but no browser-level bootstrap smoke test. Valid HTML and CSS can therefore deploy while an entry-module failure leaves `#root` empty and the public site blank.

## Possible Solution

Serve `website/dist` in CI, assert the hero renders in a browser, then block the hashed entry module and assert the static fallback remains visible.

## Minimal Reproducible Example

1. Build and serve `website/dist`.
2. Block `/assets/index-*.js` in the browser.
3. Load `/` and inspect `#root`.

Before the fallback fix, `#root` is empty and the page has no actionable content.

## Context

A blank production page was observed after a landing-page deployment. The HTML, stylesheet, and hashed entry asset each returned HTTP 200, so the existing build check could not detect the user-visible failure mode.
