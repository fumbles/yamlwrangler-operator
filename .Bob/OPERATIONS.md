# Operations Runbook

## Build

Build and test locally:

```bash
make build
/usr/bin/env GOCACHE=/private/tmp/yamlwrangler-go-build-cache GOMODCACHE=/private/tmp/yamlwrangler-go-mod-cache go test ./...
```

Build images manually:

```bash
make podman-build IMG=<registry>/<namespace>/app-dashboard-operator:<tag>
make plugin-build PLUGIN_IMG=<registry>/<namespace>/app-dashboard-console-plugin:<tag>
```

## Raw Development Deploy

```bash
./build-and-deploy.sh
```

This installs CRDs directly and applies `manifests/deploy/`.

## OLM Install

```bash
./build-and-deploy.sh v1.0.1 --olm
```

The OLM flow builds and pushes:

- operator image
- console plugin image
- bundle image
- catalog image

Then it applies:

- `manifests/olm/operatorgroup.yaml`
- generated `CatalogSource`
- generated `Subscription`

Verify:

```bash
oc get catalogsource,subscription,installplan,csv,pods -n app-dashboard-operator
oc get operator app-dashboard-operator.app-dashboard-operator -n app-dashboard-operator
```

## Dry-Run Procedure

Use this sequence before a real install or upgrade:

```bash
git diff --check
bash -n build-and-deploy.sh
/usr/bin/env GOCACHE=/private/tmp/yamlwrangler-go-build-cache GOMODCACHE=/private/tmp/yamlwrangler-go-mod-cache go test ./...
oc apply --dry-run=client --validate=false -f manifests/crds/
oc apply --dry-run=server -f manifests/deploy/
oc apply --dry-run=server -f manifests/samples/
```

If a dry run fails, fix the manifest or controller change first. Do not clean up
OLM state to work around invalid manifests.

Server dry runs require cluster access and use the live API schema. Client dry
runs are useful for parsing local CRD YAML before the new API exists in the
cluster.

## Failed OLM Catalog Troubleshooting

`exec container process /bin/opm: Exec format error` means the catalog image was
built for the wrong architecture. Build bundle/catalog images with
`--platform linux/amd64`.

`integrity check failed ... /tmp/cache/pogreb.v1/digest` means `opm serve` was
started with a persistent cache dir that was not initialized. Use:

```bash
opm serve /configs
```

## Failed CSV Troubleshooting

`CRD version not served` usually means the CSV owned CRD version was replaced
with the package version. Owned CRDs must stay at `v1alpha1`.

`Service account is owned by another ClusterServiceVersion` means a bad pending
CSV still owns install components. Delete the bad CSV and let the replacement
CSV continue:

```bash
oc delete csv <bad-csv-name> -n app-dashboard-operator
```

## Cleanup

Remove OLM install:

```bash
oc delete subscription app-dashboard-operator -n app-dashboard-operator --ignore-not-found
oc delete csv -n app-dashboard-operator -l operators.coreos.com/app-dashboard-operator.app-dashboard-operator
oc delete catalogsource app-dashboard-operator-catalog -n app-dashboard-operator --ignore-not-found
oc delete operatorgroup app-dashboard-operator -n app-dashboard-operator --ignore-not-found
```

Remove dashboard plugin instance:

```bash
oc delete appdashboard yamlwrangler --ignore-not-found
```

Remove dashboard plugin workload if needed:

```bash
oc delete namespace app-dashboard --ignore-not-found
```
