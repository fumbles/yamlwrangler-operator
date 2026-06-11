# Install from Docker Hub with OLM

This guide shows how to install the Yamlwrangler App Dashboard Operator from Docker Hub using OpenShift's Operator Lifecycle Manager (OLM), so it appears in the **Installed Operators** UI.

## Prerequisites

- OpenShift 4.x cluster
- Cluster admin permissions
- `oc` or `kubectl` CLI configured

## Installation Steps

### 1. Create CatalogSource

Apply the CatalogSource manifest:

```bash
kubectl apply -f https://raw.githubusercontent.com/fumbles/yamlwrangler-operator/main/manifests/olm/catalogsource-dockerhub.yaml
```

Or if you have the repo cloned:

```bash
kubectl apply -f manifests/olm/catalogsource-dockerhub.yaml
```

Or create it directly:

```bash
kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: yamlwrangler-catalog
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: docker.io/fumbles/yamlwrangler-operator-catalog:v1.0.5
  displayName: Yamlwrangler Operators
  publisher: Yamlwrangler
  updateStrategy:
    registryPoll:
      interval: 10m
EOF
```

### 2. Wait for CatalogSource to be Ready

```bash
oc get catalogsource yamlwrangler-catalog -n openshift-marketplace -w
```

Wait until `LAST OBSERVED STATE` shows `READY` (press Ctrl+C to exit watch).

### 3. Install from OpenShift Console

1. Navigate to **Operators → OperatorHub**
2. Search for "**Yamlwrangler**" or "**App Dashboard**"
3. Click on the **Yamlwrangler App Dashboard Operator** tile
4. Click **Install**
5. Choose installation namespace: **app-dashboard-operator** (recommended)
6. Click **Install**

### 4. Verify Installation

```bash
# Check operator appears in Installed Operators
oc get operator -n app-dashboard-operator

# Check CSV (ClusterServiceVersion)
oc get csv -n app-dashboard-operator

# Check operator pod is running
oc get pods -n app-dashboard-operator
```

Expected output:
```
NAME                                      READY   STATUS    RESTARTS   AGE
app-dashboard-operator-xxxxxxxxxx-xxxxx   1/1     Running   0          1m
```

### 5. Create AppDashboard Instance

From the OpenShift Console:
1. Navigate to **Operators → Installed Operators**
2. Click **Yamlwrangler App Dashboard Operator**
3. Click **App Dashboard** tab
4. Click **Create AppDashboard**
5. Use the default YAML or customize, then click **Create**

Or from CLI:

```bash
kubectl apply -f - <<EOF
apiVersion: dashboard.yamlwrangler.com/v1alpha1
kind: AppDashboard
metadata:
  name: yamlwrangler
spec:
  namespace: app-dashboard
  pluginName: app-dashboard
  displayName: Yamlwrangler App Dashboard
  image: docker.io/fumbles/yamlwrangler-dashboard:v1.0.5
  replicas: 2
  enableConsolePlugin: true
  consoleLink:
    enabled: true
    name: app-dashboard-link
    text: App Dashboard
    href: /app-dashboard
    section: App Dashboard
EOF
```

### 6. Verify Dashboard Plugin

```bash
# Check plugin deployment
oc get pods -n app-dashboard

# Check console plugin is registered
oc get consoleplugin app-dashboard

# Check console link
oc get consolelink app-dashboard-link
```

### 7. Enable Discovery for Your Namespaces

Label any namespace to enable automatic app discovery:

```bash
oc label namespace <your-namespace> dashboard.yamlwrangler.com/enabled=true
```

For example:
```bash
oc label namespace media dashboard.yamlwrangler.com/enabled=true
```

### 8. Access the Dashboard

1. Refresh your OpenShift Console
2. Look for **App Dashboard** in the left navigation menu
3. Click to view your discovered applications

## Upgrading

When a new version is available:

1. Update the CatalogSource image tag:
```bash
oc patch catalogsource yamlwrangler-catalog -n openshift-marketplace \
  --type merge -p '{"spec":{"image":"docker.io/fumbles/yamlwrangler-operator-catalog:v1.0.6"}}'
```

2. The operator will automatically upgrade if `installPlanApproval` is set to `Automatic`

## Uninstalling

### Remove Dashboard Instance

```bash
oc delete appdashboard yamlwrangler
```

### Uninstall Operator

From OpenShift Console:
1. Navigate to **Operators → Installed Operators**
2. Click the three dots menu next to **Yamlwrangler App Dashboard Operator**
3. Click **Uninstall Operator**

Or from CLI:

```bash
# Delete subscription
oc delete subscription app-dashboard-operator -n app-dashboard-operator

# Delete CSV
oc delete csv -n app-dashboard-operator -l operators.coreos.com/app-dashboard-operator.app-dashboard-operator

# Delete operator namespace (optional)
oc delete namespace app-dashboard-operator
```

### Remove CatalogSource

```bash
oc delete catalogsource yamlwrangler-catalog -n openshift-marketplace
```

## Troubleshooting

### CatalogSource Not Ready

```bash
# Check catalog pod logs
oc logs -n openshift-marketplace -l olm.catalogSource=yamlwrangler-catalog

# Check catalog source status
oc get catalogsource yamlwrangler-catalog -n openshift-marketplace -o yaml
```

### Operator Not Installing

```bash
# Check subscription status
oc get subscription app-dashboard-operator -n app-dashboard-operator -o yaml

# Check install plan
oc get installplan -n app-dashboard-operator

# Check CSV status
oc get csv -n app-dashboard-operator -o yaml
```

### Dashboard Not Appearing

```bash
# Check AppDashboard status
oc get appdashboard yamlwrangler -o yaml

# Check operator logs
oc logs -f deployment/app-dashboard-operator -n app-dashboard-operator

# Check plugin deployment
oc get deployment app-dashboard -n app-dashboard
```

## Support

- GitHub Issues: https://github.com/fumbles/yamlwrangler-operator/issues
- Documentation: [README.md](README.md)
- Docker Hub: https://hub.docker.com/r/fumbles/yamlwrangler-operator