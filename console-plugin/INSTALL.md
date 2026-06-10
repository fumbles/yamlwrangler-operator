# Installation Guide

## Quick Install from Docker Hub

### Prerequisites

- OpenShift cluster (4.10+)
- `kubectl` or `oc` CLI configured
- Cluster admin permissions
- Helm 3.x installed

### Install the Console Plugin

#### Option 1: Using Helm (Recommended)

```bash
# Clone the repository
git clone https://github.com/fumbles/yamlwrangler-dashboard.git
cd yamlwrangler-dashboard

# Install with Helm
helm install app-dashboard charts/openshift-console-plugin \
  --create-namespace \
  --namespace app-dashboard
```

The Helm chart will automatically use the Docker Hub image: `fumbles/yamlwrangler-dashboard:v1.0.0`

#### Option 2: Manual Installation

```bash
# Create namespace
kubectl create namespace app-dashboard

# Deploy using Helm with custom values
helm install app-dashboard charts/openshift-console-plugin \
  --namespace app-dashboard \
  --set plugin.name=app-dashboard \
  --set plugin.description="Yamlwrangler App Dashboard" \
  --set plugin.image=fumbles/yamlwrangler-dashboard:v1.0.0
```

### Add Application Menu Link

```bash
# Update the console URL in console-link.yaml to match your cluster
# Then apply:
kubectl apply -f console-link.yaml
```

**Update the href** in `console-link.yaml`:
```yaml
spec:
  href: https://<your-console-url>/app-dashboard
```

Get your console URL:
```bash
oc get route console -n openshift-console -o jsonpath='{.spec.host}'
```

### Verify Installation

```bash
# Check plugin is running
kubectl get pods -n app-dashboard

# Check ConsolePlugin resource
kubectl get consoleplugin app-dashboard

# Check logs
kubectl logs -f deployment/app-dashboard -n app-dashboard
```

### Access the Dashboard

1. Open OpenShift Console
2. Click the Application Menu (9 dots icon in top right)
3. Select "App Dashboard" under "Red Hat Applications"

Or navigate directly to: `https://<console-url>/app-dashboard`

### Enable Console Plugin (if not auto-enabled)

```bash
# Patch the console operator to enable the plugin
kubectl patch console.operator.openshift.io cluster \
  --type='json' \
  -p='[{"op": "add", "path": "/spec/plugins/-", "value": "app-dashboard"}]'
```

## Docker Image

The dashboard is available on Docker Hub:

```bash
docker pull fumbles/yamlwrangler-dashboard:v1.0.0
# or
docker pull fumbles/yamlwrangler-dashboard:latest
```

**Image**: `fumbles/yamlwrangler-dashboard:v1.0.0`

## Configuration

### Custom Image

To use a different image version:

```bash
helm upgrade app-dashboard charts/openshift-console-plugin \
  --namespace app-dashboard \
  --set plugin.image=fumbles/yamlwrangler-dashboard:latest
```

### Resource Limits

Adjust resources in `values.yaml`:

```yaml
plugin:
  resources:
    requests:
      cpu: 10m
      memory: 50Mi
    limits:
      cpu: 100m
      memory: 128Mi
```

## Uninstall

```bash
# Remove Helm release
helm uninstall app-dashboard -n app-dashboard

# Remove namespace
kubectl delete namespace app-dashboard

# Remove console link
kubectl delete consolelink app-dashboard-link

# Remove plugin from console (if manually added)
kubectl patch console.operator.openshift.io cluster \
  --type='json' \
  -p='[{"op": "remove", "path": "/spec/plugins", "value": ["app-dashboard"]}]'
```

## Next Steps

1. Install the [Operator](https://github.com/fumbles/yamlwrangler-operator) for automatic discovery
2. Label namespaces: `kubectl label namespace <ns> dashboard.yamlwrangler.com/enabled=true`
3. Customize ConfigMaps for each namespace
4. Add custom links for external services

## Troubleshooting

### Plugin Not Loading

```bash
# Check console operator config
oc get console.operator.openshift.io cluster -o yaml

# Restart console pods
oc delete pods -n openshift-console -l app=console
```

### Hard Refresh Browser

Press `Ctrl+Shift+R` (Windows/Linux) or `Cmd+Shift+R` (Mac) to clear cache.

### Check Plugin Status

```bash
# View plugin pods
kubectl get pods -n app-dashboard

# View plugin logs
kubectl logs -f deployment/app-dashboard -n app-dashboard

# Check ConsolePlugin resource
kubectl get consoleplugin app-dashboard -o yaml
```

## Support

- GitHub Issues: https://github.com/fumbles/yamlwrangler-dashboard/issues
- Documentation: [README.md](README.md)
- How-To Guide: [HOW-TO.md](HOW-TO.md)