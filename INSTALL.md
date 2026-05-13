# Installation Guide

## Quick Install from Docker Hub

### Prerequisites

- OpenShift or Kubernetes cluster
- `kubectl` or `oc` CLI configured
- Cluster admin permissions

### Install the Operator

```bash
# Apply all manifests
kubectl apply -f https://raw.githubusercontent.com/fumbles/yamlwrangler-operator/main/manifests/deploy/namespace.yaml
kubectl apply -f https://raw.githubusercontent.com/fumbles/yamlwrangler-operator/main/manifests/deploy/serviceaccount.yaml
kubectl apply -f https://raw.githubusercontent.com/fumbles/yamlwrangler-operator/main/manifests/deploy/clusterrole.yaml
kubectl apply -f https://raw.githubusercontent.com/fumbles/yamlwrangler-operator/main/manifests/deploy/clusterrolebinding.yaml
kubectl apply -f https://raw.githubusercontent.com/fumbles/yamlwrangler-operator/main/manifests/deploy/deployment.yaml
```

Or clone and apply:

```bash
git clone https://github.com/fumbles/yamlwrangler-operator.git
cd yamlwrangler-operator
kubectl apply -f manifests/deploy/
```

### Verify Installation

```bash
# Check operator is running
kubectl get pods -n app-dashboard-operator

# Check logs
kubectl logs -f deployment/app-dashboard-operator -n app-dashboard-operator
```

### Enable Discovery for a Namespace

```bash
# Label a namespace to enable auto-discovery
kubectl label namespace <your-namespace> dashboard.yamlwrangler.com/enabled=true

# Verify ConfigMap was created
kubectl get configmap dashboard-config-<your-namespace> -n <your-namespace>
```

### Customize App Settings

```bash
# Edit the generated ConfigMap
kubectl edit configmap dashboard-config-<your-namespace> -n <your-namespace>
```

See the [README](README.md) for full configuration options.

## Docker Image

The operator is available on Docker Hub:

```bash
docker pull fumbles/yamlwrangler-operator:v1.0.0
# or
docker pull fumbles/yamlwrangler-operator:latest
```

**Image**: `fumbles/yamlwrangler-operator:v1.0.0`

## Uninstall

```bash
kubectl delete -f manifests/deploy/
```

## Next Steps

1. Install the [Dashboard Plugin](https://github.com/fumbles/yamlwrangler-dashboard)
2. Label namespaces for discovery
3. Customize ConfigMaps
4. View apps in the dashboard

## Support

- GitHub Issues: https://github.com/fumbles/yamlwrangler-operator/issues
- Documentation: [README.md](README.md)