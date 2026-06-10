# Yamlwrangler Dashboard Operator Maintainer Notes

This repo is intended to be the single source tree for the dashboard operator and the OpenShift console plugin it installs.

Audience:

- A human maintainer
- Codex
- IBM Bob

## What This Project Should Do

The operator is installed once into the cluster. After that, a user creates an `AppDashboard` custom resource. The operator reconciles that CR into a working OpenShift console plugin installation.

Separately, labeled namespaces and app grouping resources control what applications appear inside the dashboard.

## Primary Install Flow

1. Install CRDs:

   ```bash
   make install
   ```

2. Deploy the operator:

   ```bash
   make deploy
   ```

3. Create the dashboard CR:

   ```bash
   kubectl apply -f manifests/samples/appdashboard.yaml
   ```

4. Enable app discovery in an application namespace:

   ```bash
   oc label namespace <namespace> dashboard.yamlwrangler.com/enabled=true
   ```

## Repository Layout

- `api/v1alpha1/`: Go API types for custom resources.
- `controllers/`: Reconciliation logic.
- `manifests/crds/`: Hand-maintained CRDs for this lightweight repo layout.
- `manifests/deploy/`: Operator deployment, RBAC, namespace, and service account.
- `manifests/samples/`: Example custom resources users can apply.
- `console-plugin/`: The OpenShift console dynamic plugin source copied from the former dashboard repo.

## Custom Resources

### AppDashboard

Cluster-scoped install resource. It owns the console plugin deployment path:

- Namespace
- ServiceAccount
- ConfigMap containing nginx config
- Service with OpenShift serving cert annotation
- Deployment serving plugin assets
- ConsolePlugin
- Optional ConsoleLink
- Optional patch to `Console.operator.openshift.io/cluster.spec.plugins`

### DashboardAppGroup

Namespaced app grouping resource. It selects deployments and applies dashboard labels/annotations.

### Namespace Label Discovery

Namespaces labeled with:

```text
dashboard.yamlwrangler.com/enabled=true
```

get a `dashboard-config-<namespace>` ConfigMap. Deployment and Route events in that namespace now enqueue reconciliation, so newly created deployments are merged into the existing configmap while existing manual edits are preserved.

## Important Maintenance Rules

- Preserve user edits in generated namespace configmaps. Only append missing apps or fill blank generated fields.
- Do not remove app entries automatically unless a retention policy is explicitly added.
- Keep the frontend data contract stable:
  - `dashboard.yamlwrangler.com/enabled=true`
  - `dashboard.yamlwrangler.com/display-name`
  - `dashboard.yamlwrangler.com/category`
  - `dashboard.yamlwrangler.com/description`
  - `dashboard.yamlwrangler.com/app-group`
  - `dashboard.yamlwrangler.com/primary-route`
  - `dashboard.yamlwrangler.com/custom-links`
- Prefer typed Kubernetes resources for core APIs and `unstructured.Unstructured` for OpenShift console APIs unless typed imports are already available and build cleanly.
- Run `gofmt` and `go build ./...` after controller/API changes.

## Current Verification Command

Use local caches if the filesystem sandbox cannot write to `~/Library/Caches` or `~/go`:

```bash
/usr/bin/env GOCACHE=/private/tmp/yamlwrangler-go-build-cache GOMODCACHE=/private/tmp/yamlwrangler-go-mod-cache go build ./...
```
