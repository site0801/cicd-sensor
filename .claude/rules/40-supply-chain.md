---
paths:
  - ".github/**"
  - ".gitlab-ci.yml"
  - "renovate.json"
  - "renovate.json5"
  - "Dockerfile*"
---

# Supply-Chain Hygiene

- GitHub Actions `uses:` is pinned to a full commit SHA.
- Allowed GitHub Actions are restricted at the **organisation level via an allowlist**; you cannot just reach for any Action. New Actions go through the org allowlist update process before they can be added to a workflow.
- Dockerfile `FROM` lines are pinned to a multi-arch index digest (`image:tag@sha256:...`), not a floating tag. Existing Dockerfiles in this repo follow this pattern (see `Dockerfile.agent`, `Dockerfile.bpf-builder`).
- Dependency updates are managed by **Renovate** (the explicitly chosen tool for this repo). Renovate config sets `minimumReleaseAge` / cooldown so newly published versions are not auto-applied immediately. Update the Renovate config rather than introducing a parallel update tool.
- Avoid adding new third-party Actions or base images without reviewing the maintainer and release model first.
