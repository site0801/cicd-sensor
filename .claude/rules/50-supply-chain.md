---
paths:
  - ".github/**"
  - ".gitlab-ci.yml"
  - "renovate.json"
  - "renovate.json5"
  - ".github/dependabot.yml"
---

# Supply-Chain Hygiene

- GitHub Actions `uses:` is pinned to a full commit SHA. Update and verify pins with `pinact`.
- Dependency auto-update tools (Dependabot, Renovate) use cooldown / `minimumReleaseAge` so newly published versions are not auto-applied immediately.
- Avoid adding new third-party Actions without reviewing the maintainer and release model first.
