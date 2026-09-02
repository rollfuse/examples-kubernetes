# rollfuse on Kubernetes

A feature-flagged checkout service deployed on Kubernetes with
[rollfuse](https://rollfuse.com)'s [Go SDK](https://github.com/rollfuse/go-sdk):
`Deployment`, `Service`, `ConfigMap`, `Secret` — no Helm, no operator,
nothing beyond plain manifests. Runs on any cluster, no rollfuse account
required.

```bash
git clone https://github.com/rollfuse/examples-kubernetes.git
cd examples-kubernetes

# Build the two images and load them into your cluster.
# (kind shown below; swap for `minikube image load` or push to a
# registry your cluster can pull from.)
docker build -t examples-kubernetes-checkout:local -f Dockerfile.checkout .
docker build -t examples-kubernetes-mock-rollfuse-api:local -f Dockerfile.mock-api .
kind load docker-image examples-kubernetes-checkout:local
kind load docker-image examples-kubernetes-mock-rollfuse-api:local

kubectl apply -k k8s/
kubectl -n rollfuse-example wait --for=condition=available --timeout=60s deployment --all

kubectl -n rollfuse-example port-forward svc/checkout 8080:8080
# in another terminal:
curl "http://localhost:8080/checkout?user=user_1"
```

## What this demonstrates

- **A flag-driven rollout that's independent of your Kubernetes rollout.**
  `checkout`'s two replicas never need a new image, a new `Deployment`
  revision, or a restart to change *which* variation subjects see —
  editing `k8s/mock-rollfuse-api-configmap.yaml`'s rollout split and
  re-applying is enough. See "Change the rollout, live" below — this was
  verified against a real cluster, not asserted.
- **Idiomatic config/secret separation**: `ROLLFUSE_API_BASE_URL` is a
  plain env var (`checkout-deployment.yaml`); `ROLLFUSE_SERVICE_CREDENTIAL`
  comes from a `Secret` (`checkout-secret.yaml`) via `secretKeyRef` —
  never a literal in the Deployment spec, so it's the one file a real
  cluster would swap for an External Secrets Operator resource or
  similar.
- **Readiness gated on the SDK actually being ready**: `checkout`'s
  `readinessProbe` hits `/readyz`, which the app only serves *after*
  `client.Start()` has completed — see `cmd/checkout/main.go`. A pod
  never receives traffic before it can actually evaluate the flag.

## Change the rollout, live

```bash
kubectl -n rollfuse-example edit configmap mock-rollfuse-config
# change "percentage": 50 to something else, or "enabled": true to false
```

Within about a minute (kubelet's ConfigMap volume sync interval, plus the
SDK's own 30s Configuration refresh — no code in this repo controls
either), `kubectl -n rollfuse-example logs -f deploy/checkout` starts
showing the new split. No `kubectl rollout restart`, no new Pod.

## How it's wired

```
k8s/checkout-deployment.yaml (2 replicas)
        │  ROLLFUSE_API_BASE_URL=http://mock-rollfuse-api:8090
        ▼
k8s/mock-rollfuse-api-deployment.yaml (1 replica)
        │  reads /config/checkout-config.json per request
        ▼
k8s/mock-rollfuse-api-configmap.yaml (mounted as a volume)
```

`mock-rollfuse-api` (`cmd/mock-rollfuse-api/main.go`) is a small stand-in
for the real rollfuse platform API — self-contained so this example needs
no rollfuse account. Point `checkout` at a real rollfuse environment
instead: put a real Service Credential in `checkout-secret.yaml`, change
`ROLLFUSE_API_BASE_URL` in `checkout-deployment.yaml`, and remove
`k8s/mock-rollfuse-api-*.yaml` from `k8s/kustomization.yaml` — the
application code doesn't change at all.

> **`k8s/checkout-secret.yaml` is a demo-only, git-committed plain
> `Secret`.** A real credential belongs in a real secret manager (your
> cloud provider's, Vault, External Secrets Operator, …) synced into a
> cluster Secret — never committed to git.

## Development

```bash
go build ./...
go vet ./...
```

`checkout` reads `ROLLFUSE_API_BASE_URL` (default
`http://localhost:8090`), `ROLLFUSE_SERVICE_CREDENTIAL` (default
`demo-credential`), `LISTEN_ADDR` (default `:8080`), and `POD_NAME`
(populated from the Downward API in `checkout-deployment.yaml`, used only
to label log lines and responses so you can tell replicas apart).
`mock-rollfuse-api` reads `CONFIG_PATH` (default
`/config/checkout-config.json`) and `LISTEN_ADDR` (default `:8090`).

## Related

- [`rollfuse/go-sdk`](https://github.com/rollfuse/go-sdk) — the SDK this
  example runs.
- [`rollfuse/examples-go-prometheus`](https://github.com/rollfuse/examples-go-prometheus) —
  the same SDK, with Prometheus metrics and a Grafana dashboard instead of
  a Kubernetes deployment.
