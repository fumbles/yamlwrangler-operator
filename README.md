[![Docker Pulls](https://img.shields.io/docker/pulls/fumbles/yamlwrangler-operator?logo=docker&label=operator%20pulls)](https://hub.docker.com/r/fumbles/yamlwrangler-operator)
[![Docker Pulls](https://img.shields.io/docker/pulls/fumbles/yamlwrangler-dashboard?logo=docker&label=dashboard%20pulls)](https://hub.docker.com/r/fumbles/yamlwrangler-dashboard)
[![Docker Pulls](https://img.shields.io/docker/pulls/fumbles/yamlwrangler-operator-bundle?logo=docker&label=bundle%20pulls)](https://hub.docker.com/r/fumbles/yamlwrangler-operator-bundle)
[![Docker Pulls](https://img.shields.io/docker/pulls/fumbles/yamlwrangler-operator-catalog?logo=docker&label=catalog%20pulls)](https://hub.docker.com/r/fumbles/yamlwrangler-operator-catalog)
[![Docker Image Version](https://img.shields.io/docker/v/fumbles/yamlwrangler-operator?sort=semver&logo=docker&label=version)](https://hub.docker.com/r/fumbles/yamlwrangler-operator)
[![App Showcase](https://img.shields.io/badge/App_Showcase-yamlwrangler.com-ee0000?logo=redhatopenshift&logoColor=white)](https://yamlwrangler.com)

# Yamlwrangler App Dashboard Operator

The Yamlwrangler App Dashboard Operator installs and manages an OpenShift console plugin that displays application cards from cluster state. It discovers apps from labeled deployments, resolves OpenShift Routes, manages dashboard ConfigMaps, and exposes operator operands for editing namespace config, app groups, and custom links from the OpenShift Installed Operators UI.

The primary install and upgrade flow is OLM-based:

1. Build the operator, console plugin, OLM bundle, and OLM catalog images.
2. Push those images to the OpenShift internal registry.
3. Install or upgrade through a `CatalogSource` and `Subscription`.
4. Create or update the `AppDashboard` instance that installs the console plugin workload.

Raw manifests still exist for development, but OLM through the generated catalog is the supported flow.

## Components

- `AppDashboard`: installs the console plugin deployment, service, `ConsolePlugin`, optional console link, and console plugin enablement.
- `DashboardNamespaceConfig`: manages the `dashboard-config-<namespace>` ConfigMap for one namespace.
- `DashboardLink`: adds or edits one custom dashboard link for an app.
- `DashboardAppGroup`: selects and groups deployments into one dashboard card.
- Console plugin: reads labeled deployments, deployment annotations, routes, and dashboard ConfigMaps to render the App Dashboard page.

Default namespaces:

- Operator namespace: `app-dashboard-operator`
- Console plugin namespace: `app-dashboard`

## Quick Start

Use a new semantic version tag for each OLM upgrade. Reusing an installed CSV version can leave OLM on the existing version.

To install from the published Docker Hub catalog on a cluster that already has OLM:

```bash
oc apply -f manifests/olm/install.yaml
```

Or apply only the catalog source and install from OperatorHub:

```bash
oc apply -f manifests/olm/catalogsource.yaml
```

Then search OperatorHub for `Yamlwrangler App Dashboard Operator` and install it into `app-dashboard-operator`.

For local development against the current cluster:

```bash
./build-and-deploy.sh v1.0.3 --olm
```

The script builds and pushes the operator and plugin images, builds a linux/amd64 OLM bundle and catalog image, applies a generated internal-registry `CatalogSource` and `Subscription`, waits for the CSV to succeed, and applies the `AppDashboard` sample with the freshly built plugin image.

Check the install:

```bash
oc get catalogsource,subscription,installplan,csv -n app-dashboard-operator
oc get catalogsource yamlwrangler-catalog -n openshift-marketplace
oc get pods -n app-dashboard-operator
oc get pods -n app-dashboard
oc get appdashboard yamlwrangler -o yaml
```

Open the OpenShift console and use the `App Dashboard` navigation item after the console plugin refreshes.

## Ship Release Images

To publish the final images to Docker Hub under `fumbles`, log in first and run the OLM shipping flow:

```bash
podman login docker.io
./build-and-deploy.sh v1.0.3 --olm --ship
```

With the default environment, `--ship --olm` pushes:

- `docker.io/fumbles/yamlwrangler-operator:v1.0.3`
- `docker.io/fumbles/yamlwrangler-dashboard:v1.0.3`
- `docker.io/fumbles/yamlwrangler-operator-bundle:v1.0.3`
- `docker.io/fumbles/yamlwrangler-operator-catalog:v1.0.3`

The local cluster install still uses the internal OpenShift registry images that were just built and pushed. The Docker Hub images are the release artifacts to reference from a public catalog or downstream install flow.

Useful overrides:

```bash
DOCKERHUB_ORG=fumbles \
DOCKERHUB_OPERATOR_IMAGE_NAME=yamlwrangler-operator \
DOCKERHUB_PLUGIN_IMAGE_NAME=yamlwrangler-dashboard \
DOCKERHUB_BUNDLE_IMAGE_NAME=yamlwrangler-operator-bundle \
DOCKERHUB_CATALOG_IMAGE_NAME=yamlwrangler-operator-catalog \
./build-and-deploy.sh v1.0.3 --olm --ship
```

## Upgrade Notes

OLM upgrades are versioned by CSV. Pick a new version for every published operator build:

```bash
./build-and-deploy.sh v1.0.4 --olm
```

The script detects the existing CSV and sets `replaces` on the new CSV and catalog channel entry when the version changes.

If OLM does not move:

```bash
oc get subscription app-dashboard-operator -n app-dashboard-operator -o yaml
oc get installplan,csv -n app-dashboard-operator
oc get catalogsource app-dashboard-operator-catalog -n app-dashboard-operator -o yaml
```

If the catalog pod fails with `Exec format error`, rebuild through the script. The catalog image is built with `--platform linux/amd64` because OpenShift must be able to run `/bin/opm` on the cluster nodes.

## Configure The Dashboard

Create an `AppDashboard` if the deployment script has not already applied one:

```yaml
apiVersion: dashboard.yamlwrangler.com/v1alpha1
kind: AppDashboard
metadata:
  name: yamlwrangler
spec:
  namespace: app-dashboard
  pluginName: app-dashboard
  displayName: Yamlwrangler App Dashboard
  image: image-registry.openshift-image-registry.svc:5000/app-dashboard/app-dashboard-console-plugin:v1.0.3
  replicas: 2
  enableConsolePlugin: true
```

The dashboard displays deployments labeled or annotated with:

```bash
oc label deployment <deployment> -n <namespace> dashboard.yamlwrangler.com/enabled=true
```

Supported deployment annotations:

- `dashboard.yamlwrangler.com/display-name`
- `dashboard.yamlwrangler.com/category`
- `dashboard.yamlwrangler.com/description`
- `dashboard.yamlwrangler.com/app-group`
- `dashboard.yamlwrangler.com/primary-route`
- `dashboard.yamlwrangler.com/custom-links`

You can manage those fields directly with deployment labels and annotations, or through the custom resources below.

## Manage A Namespace

Create a `DashboardNamespaceConfig` in an application namespace to generate or edit `dashboard-config-<namespace>`:

```yaml
apiVersion: dashboard.yamlwrangler.com/v1alpha1
kind: DashboardNamespaceConfig
metadata:
  name: media-dashboard
  namespace: media
spec:
  enabled: true
  discoveryMode: Merge
  apps:
    plex:
      enabled: true
      displayName: Plex
      category: Media
      description: Media streaming server
      primaryRoute: plex
      customLinks:
        - name: Admin
          route: plex
          description: Plex admin route
```

`discoveryMode` controls how discovered deployments are handled:

- `Merge`: keep existing config and append newly discovered deployments.
- `Replace`: rebuild managed config from discovered deployments and declared apps.
- `None`: manage only the apps declared in the custom resource.

The operator writes the namespace ConfigMap and applies dashboard labels and annotations to deployments so the console plugin can pick them up.

## Add Or Edit Links

Use `DashboardLink` for a single custom link. Set either `url` for an external target or `route` for an OpenShift Route in the same namespace.

```yaml
apiVersion: dashboard.yamlwrangler.com/v1alpha1
kind: DashboardLink
metadata:
  name: plex-docs
  namespace: media
spec:
  app: plex
  name: Documentation
  category: Media
  url: https://support.plex.tv
  description: Plex documentation
```

For a Route-backed link:

```yaml
apiVersion: dashboard.yamlwrangler.com/v1alpha1
kind: DashboardLink
metadata:
  name: plex-admin
  namespace: media
spec:
  app: plex
  name: Admin
  category: Media
  route: plex
  description: Plex admin route
```

Deleting an imported `DashboardLink` removes the backing custom link from the managed ConfigMap or standalone custom-link ConfigMap.

## Group Apps

Use `DashboardAppGroup` to group related deployments into one dashboard card:

```yaml
apiVersion: dashboard.yamlwrangler.com/v1alpha1
kind: DashboardAppGroup
metadata:
  name: media-apps
  namespace: media
spec:
  displayName: Media Apps
  category: Media
  autoLabel: true
  selector:
    matchPattern: "^(plex|sonarr|radarr).*"
```

For manual grouping in namespace config, set child apps to `groupWith: <parent-app-key>` and set the parent app's `primaryRoute` to the route that should open from the grouped card.

## Live State Import

The operator backfills operands from current cluster state so existing dashboards can be managed from Installed Operators:

- Existing `dashboard-config-<namespace>` ConfigMaps are imported as `DashboardNamespaceConfig` operands.
- Deployments already labeled or annotated with `dashboard.yamlwrangler.com/enabled=true` are imported into a `DashboardNamespaceConfig` with their current dashboard annotations.
- Standalone ConfigMaps labeled `dashboard.yamlwrangler.com/type=custom-link` are imported as `DashboardLink` operands.
- Edits to imported standalone custom-link operands sync back to the source ConfigMap.

This preserves the older ConfigMap import path while allowing new work to happen through CRDs.

Check imported state:

```bash
oc get dashboardnamespaceconfigs,dashboardlinks,dashboardappgroups -A
oc get deploy -A -l dashboard.yamlwrangler.com/enabled=true
oc get cm -A -l dashboard.yamlwrangler.com/type=custom-link
```

## Development

Prerequisites:

- Go
- `oc` or `kubectl`
- Podman
- Access to an OpenShift cluster for install testing

Build and test locally:

```bash
make build
go test ./...
bash -n build-and-deploy.sh
```

Dry-run CRD changes before deploying them:

```bash
oc apply --dry-run=client --validate=false \
  -f manifests/crds/dashboard.yamlwrangler.com_appdashboards.yaml \
  -f manifests/crds/dashboard.yamlwrangler.com_dashboardappgroups.yaml \
  -f manifests/crds/dashboard.yamlwrangler.com_dashboardnamespaceconfigs.yaml \
  -f manifests/crds/dashboard.yamlwrangler.com_dashboardlinks.yaml
```

Dry-run sample operands:

```bash
oc apply --dry-run=client --validate=false -f manifests/samples/
```

For a raw development install only:

```bash
./build-and-deploy.sh v1.0.3
```

The raw path applies CRDs and `manifests/deploy/` directly. Use `--olm` for the supported installed-operator flow.

## Troubleshooting

Check operator health:

```bash
oc get pods -n app-dashboard-operator
oc logs -f deployment/app-dashboard-operator -n app-dashboard-operator
oc get csv -n app-dashboard-operator
```

Check console plugin health:

```bash
oc get appdashboard yamlwrangler -o yaml
oc get pods -n app-dashboard
oc get consoleplugin app-dashboard -o yaml
oc get console.operator.openshift.io cluster -o jsonpath='{.spec.plugins}'
```

Check discovery inputs:

```bash
oc get deploy -A -l dashboard.yamlwrangler.com/enabled=true
oc get route -A
oc get cm -A -l dashboard.yamlwrangler.com/type=custom-link
oc get dashboardnamespaceconfigs,dashboardlinks -A
```

If a route-backed link opens the wrong target, check the app's `primaryRoute` in the `DashboardNamespaceConfig` or the `dashboard.yamlwrangler.com/primary-route` deployment annotation.

## Repository Layout

```text
.
|-- api/v1alpha1/                 # CRD Go types
|-- controllers/                  # Reconcile logic and live-state import
|-- console-plugin/               # OpenShift console plugin frontend
|-- manifests/crds/               # CRD YAML
|-- manifests/deploy/             # Raw development deployment manifests
|-- manifests/olm/                # OLM install manifests and templates
|-- manifests/samples/            # Sample custom resources
`-- build-and-deploy.sh           # Build, ship, and install helper
```

## License

Apache 2.0
