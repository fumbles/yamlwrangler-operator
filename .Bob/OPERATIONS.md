# Operations Runbook

## Build

Build the operator:

```bash
make build
```

Build the operator image:

```bash
make podman-build IMG=<registry>/<namespace>/app-dashboard-operator:<tag>
```

Build the console plugin image:

```bash
# Using default values (Docker Hub)
make plugin-build PLUGIN_IMG=fumbles/yamlwrangler-dashboard:v1.0.0

# Or with custom registry/tag
make plugin-build PLUGIN_IMG=<registry>/<namespace>/yamlwrangler-dashboard:<tag>
```

Push images:

```bash
make podman-push IMG=<registry>/<namespace>/app-dashboard-operator:<tag>
make plugin-push PLUGIN_IMG=<registry>/<namespace>/yamlwrangler-dashboard:<tag>
```

## Build, Deploy, and Ship Script

The legacy `build-and-deploy.sh` workflow now builds both images from the consolidated repo:

- Operator image from the repo root.
- Console plugin image from `console-plugin/`.

Default namespaces:

- Operator: `app-dashboard-operator`
- Dashboard/plugin workload: `app-dashboard`

Deploy to the OpenShift internal registry and cluster:

```bash
./build-and-deploy.sh
```

Use an explicit tag:

```bash
./build-and-deploy.sh v1.0.1
```

Also push the same tag to Docker Hub:

```bash
podman login docker.io
./build-and-deploy.sh v1.0.1 --ship
```

Install through OLM so OpenShift reports it as an installed operator:

```bash
./build-and-deploy.sh v1.0.1 --olm
oc get operator -n app-dashboard-operator
```

The script automatically updates the CSV version to match the provided tag (e.g., `v1.0.1` becomes CSV version `1.0.1` with name `app-dashboard-operator.v1.0.1`).

The local OLM path applies `OperatorGroup`, `ServiceAccount`, and `ClusterServiceVersion` manifests from `manifests/olm/`. If the CSV is `Pending` with `Service account does not exist`, apply:

```bash
oc apply -f manifests/olm/serviceaccount.yaml
```

If the CSV is `Pending` with `Policy rule not satisfied for service account`, apply the operator RBAC:

```bash
oc apply -f manifests/deploy/clusterrole.yaml
oc apply -f manifests/deploy/clusterrolebinding.yaml
```

Ship to Docker Hub and install through OLM:

```bash
podman login docker.io
./build-and-deploy.sh v1.0.1 --ship --olm
```

Docker Hub defaults:

- `docker.io/fumbles/yamlwrangler-operator:<tag>`
- `docker.io/fumbles/yamlwrangler-dashboard:<tag>`

Override them with:

```bash
DOCKERHUB_ORG=<org> \
DOCKERHUB_OPERATOR_IMAGE_NAME=<operator-repo> \
DOCKERHUB_PLUGIN_IMAGE_NAME=<plugin-repo> \
./build-and-deploy.sh v1.0.1 --ship
```

## Install

Install CRDs:

```bash
make install
```

Deploy the operator:

```bash
make deploy
```

Install the dashboard:

```bash
kubectl apply -f manifests/samples/appdashboard.yaml
```

If using a custom plugin image, edit `spec.image` in the `AppDashboard` CR.

## Uninstall

### Remove Current Raw Install Before OLM Testing

Use this when switching from the raw manifest install to the OLM install path.

Remove the dashboard CR first so the running operator stops recreating plugin resources:

```bash
oc delete appdashboard yamlwrangler --ignore-not-found
```

Remove the dashboard from the OpenShift Console plugin list:

```bash
PLUGINS=$(oc get console.operator.openshift.io cluster -o json | jq -c '.spec.plugins // [] | map(select(. != "app-dashboard"))')
oc patch console.operator.openshift.io cluster --type merge -p "{\"spec\":{\"plugins\":${PLUGINS}}}"
```

Remove cluster-scoped console resources:

```bash
oc delete consoleplugin app-dashboard --ignore-not-found
oc delete consolelink app-dashboard-link --ignore-not-found
```

Remove raw install namespaces and RBAC:

```bash
oc delete namespace app-dashboard-operator app-dashboard app-dashboard-plugin --ignore-not-found
oc delete clusterrole app-dashboard-operator app-dashboard-patcher --ignore-not-found
oc delete clusterrolebinding app-dashboard-operator app-dashboard-patcher --ignore-not-found
```

Leave CRDs in place when switching to OLM so existing `DashboardAppGroup` resources are preserved:

```bash
oc get crd appdashboards.dashboard.yamlwrangler.com dashboardappgroups.dashboard.yamlwrangler.com
```

For a full purge, delete the CRDs too. This deletes all `AppDashboard` and `DashboardAppGroup` custom resources:

```bash
oc delete crd appdashboards.dashboard.yamlwrangler.com dashboardappgroups.dashboard.yamlwrangler.com
```

### Remove OLM Install

Remove the dashboard CR and plugin resources first:

```bash
oc delete appdashboard yamlwrangler --ignore-not-found
PLUGINS=$(oc get console.operator.openshift.io cluster -o json | jq -c '.spec.plugins // [] | map(select(. != "app-dashboard"))')
oc patch console.operator.openshift.io cluster --type merge -p "{\"spec\":{\"plugins\":${PLUGINS}}}"
oc delete consoleplugin app-dashboard --ignore-not-found
oc delete consolelink app-dashboard-link --ignore-not-found
```

Then remove all CSVs (handles multiple versions):

```bash
oc delete csv -n app-dashboard-operator -l operators.coreos.com/app-dashboard-operator.app-dashboard-operator
# Or delete specific versions
oc get csv -n app-dashboard-operator
oc delete csv app-dashboard-operator.v0.1.0 -n app-dashboard-operator --ignore-not-found
oc delete csv app-dashboard-operator.v1.0.0 -n app-dashboard-operator --ignore-not-found
```

Finally remove OLM resources and namespaces:

```bash
oc delete operatorgroup app-dashboard-operator -n app-dashboard-operator --ignore-not-found
oc delete namespace app-dashboard-operator app-dashboard --ignore-not-found
```

### Clean Up Failed OLM Upgrades

If you have duplicate CSVs (e.g., v0.1.0 "Replacing" and v1.0.0 "Pending"), delete all CSVs and reinstall:

```bash
# Delete all operator CSVs
oc delete csv -n app-dashboard-operator --all

# Wait a moment, then reinstall with the desired version
./build-and-deploy.sh v1.0.0 --olm
```

## Enable Namespace Discovery

There are two ways to enable namespace discovery:

### Method 1: Label the Namespace (Manual)

```bash
oc label namespace <namespace> dashboard.yamlwrangler.com/enabled=true
```

### Method 2: Create a DashboardAppGroup (UI-Driven)

Create a `DashboardAppGroup` CR in the namespace through the OpenShift UI or CLI:

```bash
kubectl apply -f manifests/samples/dashboardappgroup.yaml
```

Or create directly:

```yaml
apiVersion: dashboard.yamlwrangler.com/v1alpha1
kind: DashboardAppGroup
metadata:
  name: my-apps
  namespace: my-namespace
spec:
  displayName: My Applications
  category: Services
  autoLabel: true
  selector:
    matchPattern: "^my-app-.*"
```

**What happens automatically when you create a DashboardAppGroup:**
1. The operator labels the namespace with `dashboard.yamlwrangler.com/enabled=true`
2. Creates `dashboard-config-<namespace>` ConfigMap if it doesn't exist
3. The namespace controller discovers deployments and populates the ConfigMap
4. Matched deployments are labeled with `dashboard.yamlwrangler.com/enabled=true`
5. The console plugin picks up the labeled deployments

**Expected result (both methods):**

- The operator creates `dashboard-config-<namespace>` in that namespace.
- The configmap contains discovered deployments under `data.config.yaml`.
- The ConfigMap controller labels enabled deployments with `dashboard.yamlwrangler.com/enabled=true`.
- The console plugin picks up the labeled deployments.

## Add a New App

Deploy the app normally into a labeled namespace. The namespace controller watches deployment and route events and should merge the new deployment into `dashboard-config-<namespace>`.

Check:

```bash
oc get configmap dashboard-config-<namespace> -n <namespace> -o yaml
oc get deployment -n <namespace> --show-labels
```

## Troubleshooting

Check operator status:

```bash
oc get pods -n app-dashboard-operator
oc logs -n app-dashboard-operator deploy/app-dashboard-operator
```

Check dashboard install CR:

```bash
oc get appdashboard yamlwrangler -o yaml
```

Check plugin resources:

```bash
oc get namespace app-dashboard
oc get deployment,service,configmap -n app-dashboard
oc get consoleplugin app-dashboard -o yaml
oc get console.operator.openshift.io cluster -o jsonpath='{.spec.plugins}'
```

If the plugin pods are not ready, check for:

- Missing serving cert secret: the service annotation should create `<pluginName>-cert`.
- Bad plugin image in `AppDashboard.spec.image`.
- Console operator not listing the plugin under `spec.plugins`.

## CRD Maintenance

CRDs are currently hand-maintained in `manifests/crds/`. If controller-gen is later added, make sure generated CRDs preserve:

- `AppDashboard` as cluster-scoped.
- `DashboardAppGroup` as namespaced.
- Status subresources for both.

## Known Follow-Ups

- Add owner references for namespaced resources when practical. Cluster-scoped owners cannot own namespaced children directly, so this may require labels plus cleanup logic instead.
- Add envtest or controller-runtime unit tests around namespace config merge behavior.
- Consider OLM bundle packaging after the CR/controller model settles.
