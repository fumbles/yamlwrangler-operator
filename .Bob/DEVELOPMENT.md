# Development Guide

## Test deploy (isolated namespace, safe alongside OLM install)

`build-and-deploy-test.sh` deploys the operator into a separate namespace so it doesn't
touch the OLM-managed or production install. It patches the hardcoded namespaces and
RBAC names in the static manifests on the fly.

```bash
./build-and-deploy-test.sh [tag]
```

All values have defaults — override with env vars:

```bash
OPERATOR_NS=my-op-test PLUGIN_NS=my-plugin-test CR_NAME=my-test \
  ./build-and-deploy-test.sh dev-test
```

| Env var | Default | Purpose |
|---|---|---|
| `OPERATOR_NS` | `dashboard-op-test` | Namespace the operator runs in |
| `PLUGIN_NS` | `dashboard-test` | Namespace the console plugin runs in |
| `PLUGIN_NAME` | `app-dashboard-test` | `ConsolePlugin` resource name — must be unique cluster-wide to avoid colliding with the production install |
| `CR_NAME` | `test-dashboard` | `AppDashboard` CR name |
| `REGISTRY` | `image-registry.openshift-image-registry.svc:5000` | Internal registry (cluster-side image ref) |
| `PUSH_REGISTRY` | `default-route-openshift-image-registry.apps.sno.yamlwrangler.com` | External registry (push target) |

### Teardown

```bash
OPERATOR_NS=dashboard-op-test
kubectl delete appdashboard test-dashboard --ignore-not-found
kubectl delete namespace dashboard-test --ignore-not-found
kubectl delete namespace "${OPERATOR_NS}" --ignore-not-found
kubectl delete clusterrole "app-dashboard-operator-${OPERATOR_NS}" --ignore-not-found
kubectl delete clusterrolebinding "app-dashboard-operator-${OPERATOR_NS}" --ignore-not-found
```

---

## Quick checks

```bash
go build ./...
go vet ./...
```

Run before every deploy. Both must produce no output.

## How the deploy script works

`build-and-deploy.sh` always uses the static manifests in `manifests/deploy/` and
`manifests/samples/`. The operator namespace (`app-dashboard-operator`), the plugin
namespace (`app-dashboard`), and all RBAC names are **hardcoded in those files** — the
env vars `OPERATOR_NAMESPACE`, `PLUGIN_NAMESPACE`, etc. only affect the image push
coordinates and the `kubectl set image` / `kubectl rollout status` target, not where the
manifests actually land.

`APP_DASHBOARD_NAME` only controls which name is passed to `kubectl apply` for the
`AppDashboard` sample CR — it does not change the plugin's `spec.namespace`.

## Safe way to test a local build without touching the OLM install

The OLM-managed operator (CSV) owns nothing in the `manifests/deploy/` layer — the raw
deployment in `app-dashboard-operator` is what the script manages. Update just that
deployment's image:

```bash
# 1. Build & push the operator image with a test tag
TAG=dev-test ./build-and-deploy.sh dev-test
```

This will:
- build the Go binary
- build and push the operator image to the internal registry with the `dev-test` tag
- build and push the plugin image
- re-apply CRDs (safe, no-op if unchanged)
- re-apply `manifests/deploy/` (no-op if already present)
- run `kubectl set image deployment/app-dashboard-operator manager=<new-image> -n app-dashboard-operator`
- wait for rollout

Your OLM CatalogSource/Subscription/CSV are untouched because the raw deployment is a
separate object from the OLM-managed one. If OLM is managing the deployment you will
see a conflict — in that case use `make run` instead (see below).

### Checking for OLM ownership before deploying raw

```bash
kubectl get deployment app-dashboard-operator -n app-dashboard-operator \
  -o jsonpath='{.metadata.ownerReferences}' | grep -i csv
```

If that returns output, OLM owns the deployment and `kubectl set image` will be
immediately reverted. Use `make run` instead.

## Iterating locally without pushing an image

Run the controller directly against your current kubeconfig. Requires the CRDs to
already be installed:

```bash
make install   # only needed once, or after CRD changes
make run
```

This skips the image build/push/rollout cycle entirely. Leader election is disabled in
`main.go` by default (`--leader-elect=false`). The controller uses whatever
`kubectl config current-context` points at. Press `Ctrl-C` to stop.

## AppDashboard and the console plugin Route

When you apply the `AppDashboard` CR the operator creates a Deployment + Service for the
console plugin. OpenShift's service-cert operator injects a TLS cert via the
`service.alpha.openshift.io/serving-cert-secret-name` annotation — this happens
automatically. The **Route** is **not** created by the operator; it is part of the
ConsolePlugin contract and is managed by the OpenShift console operator once the plugin
is enabled.

If the plugin page is unreachable after a fresh deploy, check:

```bash
# Plugin deployment healthy?
kubectl get pods -n app-dashboard

# ConsolePlugin registered?
oc get consoleplugin app-dashboard -o yaml

# Plugin enabled in the Console operator?
oc get console cluster -o jsonpath='{.spec.plugins}'

# AppDashboard status
kubectl get appdashboard yamlwrangler -o yaml
```

## Teardown a test image rollout

Rolling back to the previous image:

```bash
kubectl rollout undo deployment/app-dashboard-operator -n app-dashboard-operator
```

Or pin to a specific tag:

```bash
kubectl set image deployment/app-dashboard-operator \
  manager=image-registry.openshift-image-registry.svc:5000/app-dashboard-operator/app-dashboard-operator:<previous-tag> \
  -n app-dashboard-operator
```
