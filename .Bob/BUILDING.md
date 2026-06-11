# Building And Publishing

## Local Checks

```bash
make build
/usr/bin/env GOCACHE=/private/tmp/yamlwrangler-go-build-cache GOMODCACHE=/private/tmp/yamlwrangler-go-mod-cache go test ./...
```

## Dry Runs

Use dry runs before touching a live install when changing CRDs, RBAC, OLM
metadata, or generated samples.

Validate CRDs parse and can be accepted by the API server:

```bash
oc apply --dry-run=client --validate=false -f manifests/crds/
```

Validate raw deploy manifests without persisting them:

```bash
oc apply --dry-run=server -f manifests/deploy/
```

Validate samples after API changes:

```bash
oc apply --dry-run=server -f manifests/samples/
```

Check the generated OLM templates without installing:

```bash
bash -n build-and-deploy.sh
```

For a full OLM package dry run, build into a temporary directory and inspect the
generated CSV, bundle metadata, and catalog YAML before pushing images. Keep the
owned CRD versions at `v1alpha1`; only the CSV package version should change.

## Raw Build And Deploy

```bash
./build-and-deploy.sh
```

## OLM Build And Deploy

```bash
./build-and-deploy.sh v1.0.1 --olm
```

The script generates temporary bundle and catalog content, builds images, pushes
them to the OpenShift registry, then installs through `CatalogSource` and
`Subscription`.

## Docker Hub Ship

Ship all images (operator, plugin, bundle, catalog) to Docker Hub:

```bash
podman login docker.io
./build-and-deploy.sh v1.0.1 --ship --olm
```

This pushes to Docker Hub:
- `docker.io/fumbles/yamlwrangler-operator:v1.0.1`
- `docker.io/fumbles/yamlwrangler-dashboard:v1.0.1`
- `docker.io/fumbles/yamlwrangler-operator-bundle:v1.0.1`
- `docker.io/fumbles/yamlwrangler-operator-catalog:v1.0.1`

Override Docker Hub names:

```bash
DOCKERHUB_ORG=<org> \
DOCKERHUB_OPERATOR_IMAGE_NAME=<operator-image> \
DOCKERHUB_PLUGIN_IMAGE_NAME=<plugin-image> \
DOCKERHUB_BUNDLE_IMAGE_NAME=<bundle-image> \
DOCKERHUB_CATALOG_IMAGE_NAME=<catalog-image> \
./build-and-deploy.sh v1.0.1 --ship --olm
```

**Note**: `--ship` without `--olm` only pushes operator and plugin images. Use `--ship --olm` together to push all 4 images needed for OLM installation from Docker Hub.
