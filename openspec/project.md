# Project Context

## What This Repo Is

An example repository under the `rollfuse` GitHub organization: a
feature-flagged demo checkout service showing
[`@rollfuse/go-sdk`](https://github.com/rollfuse/go-sdk) deployed on
Kubernetes with plain manifests (`Deployment`, `Service`, `ConfigMap`,
`Secret`), runnable end-to-end against any cluster (`kind` in CI) with no
rollfuse account required (a bundled mock, config-driven via a ConfigMap,
stands in for the platform API). It is not part of the rollfuse platform
itself.

## Repository Shape

```text
/
├── cmd/
│   ├── checkout/            The demo service: evaluates a flag per request,
│   │                        logs the outcome, exposes /healthz and /readyz.
│   └── mock-rollfuse-api/   Self-contained stand-in for the real platform
│                            API — re-reads its Configuration from disk on
│                            every GET /v1/config, so a ConfigMap edit
│                            takes effect without a restart.
└── k8s/
    ├── namespace.yaml
    ├── mock-rollfuse-api-configmap.yaml   The rollout config an editor changes live.
    ├── mock-rollfuse-api-deployment.yaml, -service.yaml
    ├── checkout-secret.yaml               DEMO-ONLY plain Secret.
    ├── checkout-deployment.yaml, -service.yaml
    └── kustomization.yaml                  `kubectl apply -k k8s/`
```

## Working On This Repo

- Keep it runnable with `kubectl apply -k k8s/` after building and
  loading two images — never add a step that requires manual setup (an
  account, a token, a config edit) before the demo works.
- `go build ./...`, `go vet ./...`, and `gofmt -l .` (clean) are the only
  static checks; correctness beyond that is verified by the CI job that
  deploys to a real `kind` cluster and exercises the live-ConfigMap-update
  claim end-to-end (`.github/workflows/ci.yml`) — this is a demo, not a
  library, so "does the documented flow actually work" is the bar, not
  unit tests.
- `mock-rollfuse-api` re-reading its config file per-request (not caching
  at startup) is load-bearing for this repo's whole point — don't
  "optimize" that back to a cache without re-verifying the live-update
  demo still works.
- The README's "Change the rollout, live" section and its timing claims
  were verified against real kubelet ConfigMap-sync + SDK refresh
  behavior on a real cluster, not assumed — keep them accurate if the
  demo's config-refresh interval or mount setup changes.
- OpenSpec here is for planning nontrivial changes to the demo itself,
  not for specifying rollfuse platform behavior (that lives in
  `rollfuse/rollfuse` and `rollfuse/go-sdk`).
