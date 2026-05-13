# App Dashboard Operator

A Kubernetes operator that automatically manages application discovery and configuration for the App Dashboard Console Plugin.

## Overview

The App Dashboard Operator provides two main controllers:

1. **Namespace Controller**: Watches labeled namespaces and auto-generates ConfigMaps with all deployments
2. **ConfigMap Controller**: Processes ConfigMaps to resolve route names to full URLs for custom links

## Features

- **Automatic Discovery**: Label a namespace and all deployments are automatically discovered
- **ConfigMap Generation**: Creates `dashboard-config-<namespace>` ConfigMaps with deployment templates
- **Route Resolution**: Automatically resolves OpenShift route names to full URLs
- **Custom Links**: Support for multiple routes per deployment (sidecars, additional services)
- **Description Field**: Custom descriptions for each custom link
- **Real-time Updates**: Watches for changes and updates ConfigMaps automatically

## Screenshots


<img width="1668" height="866" alt="image" src="https://github.com/user-attachments/assets/8b398e87-d7f5-4acf-a469-3a57cf4c4f57" />

## Quick Start

### 1. Label a Namespace

```bash
# Enable dashboard discovery for a namespace
oc label namespace media dashboard.yamlwrangler.com/enabled=true
```

The operator will automatically:
- Discover all deployments in the namespace
- Create a ConfigMap named `dashboard-config-media`
- Populate it with all deployment names as templates

### 2. Customize the ConfigMap

```bash
# Edit the generated ConfigMap
oc edit configmap dashboard-config-media -n media
```

Example ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: dashboard-config-media
  namespace: media
data:
  apps.yaml: |
    # App configuration for media namespace
    
    plex:
      enabled: true
      displayName: Plex Media Server
      category: Media
      description: Media streaming server
      primaryRoute: plex
    
    vpn-firefox:
      enabled: true
      displayName: vpn-Firefox
      category: Media
      description: vpn-backed Firefox with gluetun
      primaryRoute: vpn-firefox
      customLinks:
        - name: vpn-metube
          route: vpn-metube
          description: Metube sidecar for downloading
```

### 3. Custom Links with Route Resolution

The operator automatically resolves route names to full URLs:

**Before (what you write):**
```yaml
customLinks:
  - name: vpn-metube
    route: vpn-metube
    description: Metube sidecar
```

**After (operator resolves):**
```yaml
customLinks:
  - name: vpn-metube
    url: https://vpn-metube-media.apps.sno.yamlwrangler.com
    description: Metube sidecar
```

## Build and Deploy

### Quick Build and Deploy

```bash
# Build and deploy with auto-generated timestamp tag
cd app-dashboard-operator
./build-and-deploy.sh
```

The script automatically:
1. Builds the Go binary
2. Creates a container image with timestamp tag (e.g., `v1.0.0-20260513040330`)
3. Pushes to OpenShift internal registry
4. Deploys the operator
5. Waits for rollout to complete

### Custom Tag

```bash
# Build with a specific tag
./build-and-deploy.sh my-custom-tag
```

### Manual Build Steps

```bash
# Build the binary
make build

# Build the image
TAG=v1.0.0-$(date +%Y%m%d%H%M%S)
podman build -t default-route-openshift-image-registry.apps.sno.yamlwrangler.com/app-dashboard-operator/app-dashboard-operator:$TAG .

# Login to registry
oc registry login

# Push the image
podman push default-route-openshift-image-registry.apps.sno.yamlwrangler.com/app-dashboard-operator/app-dashboard-operator:$TAG

# Deploy
kubectl apply -f manifests/deploy/
```

## Check Operator Status

### View Image Streams

```bash
# List all operator image tags
oc get imagestreamtags -n app-dashboard-operator

# Example output:
# NAME                            IMAGE REFERENCE                                                                                                                                                          UPDATED
# app-dashboard-operator:v1.0.0-20260513040330   image-registry.openshift-image-registry.svc:5000/app-dashboard-operator/app-dashboard-operator@sha256:9324bffe83939265be4d64f44ee83e4085d0f74d4a23285b34a9dd8f1aab4b22   28 minutes ago
```

### Check Currently Running Image

```bash
# Get the image currently running in the deployment
oc get deployment app-dashboard-operator -n app-dashboard-operator -o jsonpath='{.spec.template.spec.containers[0].image}'

# Example output:
# default-route-openshift-image-registry.apps.sno.yamlwrangler.com/app-dashboard-operator/app-dashboard-operator:v1.0.0-20260513040330
```

### Check Operator Pods

```bash
# View operator pods
oc get pods -n app-dashboard-operator

# View operator logs
oc logs -f deployment/app-dashboard-operator -n app-dashboard-operator

# Check for errors
oc logs deployment/app-dashboard-operator -n app-dashboard-operator | grep -i error
```

### Check Operator Health

```bash
# Check deployment status
oc get deployment app-dashboard-operator -n app-dashboard-operator

# Check rollout status
oc rollout status deployment/app-dashboard-operator -n app-dashboard-operator

# View recent events
oc get events -n app-dashboard-operator --sort-by='.lastTimestamp'
```

## How It Works

### Namespace Controller

1. Watches for namespaces with label `dashboard.yamlwrangler.com/enabled=true`
2. Lists all deployments in the namespace
3. Creates/updates ConfigMap `dashboard-config-<namespace>`
4. Populates ConfigMap with deployment templates

### ConfigMap Controller

1. Watches ConfigMaps with name pattern `dashboard-config-*`
2. Parses the `apps.yaml` data
3. For each app with `customLinks`:
   - If `route` field is present, looks up the OpenShift route
   - Resolves route to full URL (with protocol)
   - Updates ConfigMap with resolved URL
4. Preserves manual `url` fields (doesn't overwrite)

## Configuration Fields

### ConfigMap Structure

```yaml
data:
  apps.yaml: |
    <deployment-name>:
      enabled: true|false
      displayName: string
      category: string
      description: string
      primaryRoute: string
      customLinks:
        - name: string
          route: string        # Route name (auto-resolved)
          url: string          # Direct URL (manual)
          description: string  # Custom description
```

### Custom Link Resolution

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Link identifier |
| `route` | string | No* | Route name - operator resolves to URL |
| `url` | string | No* | Direct URL - used as-is |
| `description` | string | No | Custom description text |

*Either `route` or `url` must be provided

## Troubleshooting

### Operator Not Running

```bash
# Check operator deployment
oc get deployment -n app-dashboard-operator

# Check operator logs
oc logs -f deployment/app-dashboard-operator -n app-dashboard-operator

# Check for image pull errors
oc describe pod -n app-dashboard-operator -l app=app-dashboard-operator
```

### ConfigMap Not Created

```bash
# Verify namespace label
oc get namespace media --show-labels

# Check operator logs for errors
oc logs deployment/app-dashboard-operator -n app-dashboard-operator | grep media

# Manually trigger by re-labeling
oc label namespace media dashboard.yamlwrangler.com/enabled=true --overwrite
```

### Routes Not Resolving

```bash
# Check if route exists
oc get route vpn-metube -n media

# Check operator has RBAC permissions
oc get clusterrole app-dashboard-operator-role -o yaml

# Check operator logs for route resolution
oc logs deployment/app-dashboard-operator -n app-dashboard-operator | grep "route"
```

### ConfigMap Not Updating

```bash
# Check ConfigMap watch is active
oc logs deployment/app-dashboard-operator -n app-dashboard-operator | grep "ConfigMap"

# Force update by editing ConfigMap
oc edit configmap dashboard-config-media -n media

# Restart operator if needed
oc rollout restart deployment/app-dashboard-operator -n app-dashboard-operator
```

## Development

### Prerequisites

- Go 1.21+
- OpenShift CLI (`oc`)
- Podman or Docker

### Local Development

```bash
# Install dependencies
go mod download

# Run locally (requires kubeconfig)
make run

# Build binary
make build

# Run tests
go test ./...
```

### Code Structure

```
app-dashboard-operator/
├── main.go                          # Operator entry point
├── controllers/
│   ├── namespace_controller.go      # Namespace watch & ConfigMap generation
│   └── configmap_controller.go      # ConfigMap watch & route resolution
├── api/
│   └── v1alpha1/
│       └── dashboardappgroup_types.go  # CRD types (future use)
├── manifests/
│   ├── crd-dashboardappgroup.yaml   # CRD definition (future use)
│   └── deploy/                      # Operator deployment manifests
└── build-and-deploy.sh              # Build and deploy script
```

## RBAC Permissions

The operator requires these permissions:

```yaml
# Namespace permissions
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list", "watch"]

# ConfigMap permissions
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]

# Deployment permissions
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "watch"]

# Route permissions (OpenShift)
- apiGroups: ["route.openshift.io"]
  resources: ["routes"]
  verbs: ["get", "list", "watch"]
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    App Dashboard Operator                    │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────────────┐      ┌──────────────────────┐    │
│  │ Namespace Controller │      │ ConfigMap Controller │    │
│  │                      │      │                      │    │
│  │ • Watch namespaces   │      │ • Watch ConfigMaps   │    │
│  │ • List deployments   │      │ • Parse apps.yaml    │    │
│  │ • Create ConfigMaps  │      │ • Resolve routes     │    │
│  │ • Generate templates │      │ • Update URLs        │    │
│  └──────────────────────┘      └──────────────────────┘    │
│           │                              │                   │
└───────────┼──────────────────────────────┼──────────────────┘
            │                              │
            ▼                              ▼
    ┌───────────────┐            ┌──────────────────┐
    │  Namespaces   │            │   ConfigMaps     │
    │  (labeled)    │            │ dashboard-config │
    └───────────────┘            └──────────────────┘
                                          │
                                          ▼
                                 ┌─────────────────┐
                                 │ Console Plugin  │
                                 │   (reads CMs)   │
                                 └─────────────────┘
```

## Related Projects

- [App Dashboard Console Plugin](../app-dashboard-console-plugin/) - The UI component

## License

Apache 2.0


