---
title: 'Local Agentation override can leak into the committed lockfile'
severity: 'minor'
---

# Expected Behavior

Local Agentation development should not make the committed website lockfile depend on a sibling checkout.

# Current Behavior

A file dependency can leave package-lock.json linked outside the repository after package.json returns to the registry version.

# Possible Solution

Document a local-only linking workflow and require a clean npm ci check before committing package-lock.json.

# Minimal Reproducible Example

Set website Agentation to a sibling file dependency, install it, restore the registry dependency, then update only the lockfile while the link remains installed.

# Context

The external link passed local builds but failed clean-checkout TypeScript builds because the sibling package was absent.
