# Maintainer Instructions

## Editing Rules

- Preserve user-managed ConfigMap entries.
- Prefer typed CRDs for user-facing management.
- Keep OLM owned APIs in sync with CRDs and Go types.
- Keep raw RBAC and CSV RBAC in sync.
- Keep `manifests/samples/` updated when adding user-facing fields.

## After API Changes

Update all of these manually unless controller-gen is added to the repo:

- `api/v1alpha1/*_types.go`
- `api/v1alpha1/zz_generated.deepcopy.go`
- `manifests/crds/*.yaml`
- `manifests/deploy/clusterrole.yaml`
- `manifests/olm/clusterserviceversion.yaml.template`
- `build-and-deploy.sh` catalog GVK properties
- `manifests/samples/*.yaml`
- `.Bob/*.md`

Run:

```bash
gofmt -w api/v1alpha1 controllers main.go
/usr/bin/env GOCACHE=/private/tmp/yamlwrangler-go-build-cache GOMODCACHE=/private/tmp/yamlwrangler-go-mod-cache go test ./...
```

## Code Quality Checklist

- Keep controllers idempotent: every reconcile should be safe to run repeatedly
  against the same object.
- Preserve user-authored ConfigMap data unless the API explicitly documents a
  replace mode.
- Validate required user intent at both levels when possible: CRD schema and
  controller status.
- Keep Go types, deepcopy methods, CRDs, RBAC, CSV owned APIs, catalog GVK
  properties, and samples in lockstep.
- Prefer small CRDs that map to real user tasks instead of exposing the entire
  internal ConfigMap shape as one large object.
- Update status conditions with actionable reasons when reconciliation fails.
- Run `gofmt`, `go test ./...`, `bash -n build-and-deploy.sh`, `git diff
  --check`, and CRD dry runs before calling a change ready.
- When touching OLM, verify both install surfaces: raw manifests and the
  CatalogSource/Subscription path.
