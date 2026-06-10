[![Docker Pulls](https://img.shields.io/docker/pulls/fumbles/yamlwrangler-dashboard?logo=docker)](https://hub.docker.com/r/fumbles/yamlwrangler-dashboard)
[![Docker Image Version](https://img.shields.io/docker/v/fumbles/yamlwrangler-dashboard?sort=semver&logo=docker)](https://hub.docker.com/r/fumbles/yamlwrangler-dashboard)

# App Dashboard Console Plugin

An OpenShift Console dynamic plugin that provides a unified dashboard view of all applications across namespaces.

<table>
  <tr>
    <td><img src="https://github.com/user-attachments/assets/2b574a3e-3298-4e7b-8d89-4221c07ec7bc" height="500"></td>
    <td><img src="https://github.com/user-attachments/assets/f59d2f5e-1bea-4144-8a0f-95bea50d9462" height="500"></td>
    <td><img src="https://github.com/user-attachments/assets/cc9a7ca8-2298-480d-92b7-1c3c4d4714a7" height="500"></td>
  </tr>
</table>


## Features

- **Namespace-based Discovery**: Automatically discovers deployments in labeled namespaces
- **ConfigMap-driven Configuration**: Single source of truth for app settings per namespace
- **Custom Links**: Support for multiple routes per deployment (e.g., sidecars)
- **Route Resolution**: Automatically resolves OpenShift route names to full URLs
- **Multiple Views**: Compact, Namespace, and App-grouped views
- **Category Filtering**: Organize apps by category (Media, Development, Services, etc.)

## Quick Start

### 1. Label a Namespace for Discovery

```bash
# Label a namespace to enable auto-discovery
oc label namespace media dashboard.yamlwrangler.com/enabled=true
```

The operator will automatically create a ConfigMap named `dashboard-config-<namespace>` with all deployments.

### 2. Customize App Settings

Edit the generated ConfigMap to customize app display:

```bash
oc edit configmap dashboard-config-media -n media
```

### ConfigMap Template

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: dashboard-config-media
  namespace: media
data:
  apps.yaml: |
    # App configuration for media namespace
    # Edit this file to customize how apps appear in the dashboard
    
    plex:
      enabled: true
      displayName: Plex Media Server
      category: Media
      description: Media streaming server
      primaryRoute: plex
    
    sonarr:
      enabled: true
      displayName: Sonarr
      category: Media
      description: TV show management
      primaryRoute: sonarr
    
    # Example with custom links (for deployments with sidecars)
    vpn-firefox:
      enabled: true
      displayName: vpn-Firefox
      category: Media
      description: vpn-backed Firefox
      primaryRoute: vpn-firefox
      customLinks:
        - name: vpn-metube
          route: vpn-metube
          description: vpn-metube sidecar to use gluetun
```

### Custom Links

Custom links allow you to show multiple routes for a single deployment (useful for sidecars):

**Route-based (recommended):**
```yaml
customLinks:
  - name: vpn-metube
    route: vpn-metube              # Route name - operator resolves to full URL
    description: Metube sidecar    # Optional custom description
```

**URL-based (manual):**
```yaml
customLinks:
  - name: Additional Service
    url: https://example.com
    description: External service  # Optional custom description
```

### 3. Add Standalone Custom Links (External Links)

For external services or links not tied to a deployment, use the `add-custom-link.sh` script:

```bash
# Usage: ./add-custom-link.sh <namespace> <name> "<display-name>" "<category>" "<description>" "<url>"

# Example: Add Wyze Cameras link
./add-custom-link.sh media wyze-cameras "Wyze Cameras" "Links" "Wyze Cameras streams" "https://my.wyze.com/live"

# Example: Add external monitoring link
./add-custom-link.sh monitoring grafana-cloud "Grafana Cloud" "Monitoring" "Cloud-hosted Grafana" "https://grafana.example.com"
```

**Available Categories:**
- Infrastructure
- Services
- Media
- AI / Experimental
- Development
- Monitoring
- Games
- Links (for external/misc links)

**To remove a custom link:**
```bash
oc delete configmap wyze-cameras -n media
```

### 4. Label Routes (Optional)

For better route matching, label your routes:

```bash
# Label a route to associate it with a deployment
oc label route plex dashboard.yamlwrangler.com/deployment=plex -n media
```

Or use the provided script:

```bash
./label-routes.sh media
```

## Build and Deploy

### Quick Build

```bash
# Build and deploy with auto-generated timestamp tag
npm run build && ./build-tag.sh
```

The build script automatically:
1. Builds the plugin assets with `yarn build`
2. Creates a Docker image with timestamp tag (e.g., `amd64-0.0.1-20260513040330`)
3. Pushes to OpenShift internal registry
4. Updates Helm deployment with new image
5. Restarts console pods

### Custom Tag

```bash
# Build with a specific tag
./build-tag.sh my-custom-tag
```

### Manual Build Steps

```bash
# Set variables
TAG=amd64-$(date +%Y%m%d%H%M%S)
REGISTRY=$(oc get route default-route -n openshift-image-registry -o jsonpath='{.spec.host}')
TOKEN=$(oc whoami -t)

# Rebuild plugin assets
yarn build

# Build amd64 image
podman build --platform linux/amd64 \
  -t app-dashboard-console-plugin:$TAG .

# Login to OpenShift registry
podman login -u "$(oc whoami)" -p "$TOKEN" \
  --tls-verify=false "$REGISTRY"

# Tag image
podman tag app-dashboard-console-plugin:$TAG \
  "$REGISTRY/app-dashboard/app-dashboard-console-plugin:$TAG"

# Push image
podman push --tls-verify=false \
  "$REGISTRY/app-dashboard/app-dashboard-console-plugin:$TAG"

# Upgrade Helm deployment
helm upgrade app-dashboard \
  ./charts/openshift-console-plugin \
  -n app-dashboard \
  --set plugin.name=app-dashboard \
  --set plugin.description="Yamlwrangler App Dashboard" \
  --set plugin.image=image-registry.openshift-image-registry.svc:5000/app-dashboard/app-dashboard-console-plugin:$TAG
```

## Add Application Menu Link

To make the dashboard accessible from the OpenShift Application Menu, apply the ConsoleLink:

```bash
# Apply the console link
oc apply -f console-link.yaml
```

This creates a link in the Application Menu under "Red Hat Applications" section.

**console-link.yaml:**
```yaml
apiVersion: console.openshift.io/v1
kind: ConsoleLink
metadata:
  name: app-dashboard-link
spec:
  location: ApplicationMenu
  text: App Dashboard
  href: https://console-openshift-console.apps.<domain.tld>/app-dashboard
  applicationMenu:
    section: Red Hat Applications
    imageURL: "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iNDgiIGhlaWdodD0iNDgiIHZpZXdCb3g9IjAgMCA0OCA0OCIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj48ZyBmaWxsPSJub25lIiBzdHJva2U9IiMxNTE1MTUiIHN0cm9rZS13aWR0aD0iMiI+PHBhdGggZD0iTTE0IDM0VjE0aDIwdjIwSDE0eiIvPjxwYXRoIGQ9Ik0xMCAzMFYxMGgyMCIgc3Ryb2tlPSIjNkE2QTZBIi8+PHBhdGggZD0iTTYgMjZWNmgyMCIgc3Ryb2tlPSIjRTAwMDAwIi8+PGNpcmNsZSBjeD0iMjgiIGN5PSIxOCIgcj0iMS41IiBmaWxsPSIjRTAwMDAwIiBzdHJva2U9Im5vbmUiLz48Y2lyY2xlIGN4PSIzMiIgY3k9IjE4IiByPSIxLjUiIGZpbGw9IiMwMDY2Q0MiIHN0cm9rZT0ibm9uZSIvPjwvZz48L3N2Zz4="
```

**Update the href** to match your cluster's console URL:
```bash
# Get your console URL
oc get route console -n openshift-console -o jsonpath='{.spec.host}'

# Update console-link.yaml with:
# href: https://<your-console-url>/app-dashboard
```

**To remove the link:**
```bash
oc delete consolelink app-dashboard-link
```

## Check Running Version

### View Image Streams

```bash
# List all plugin image tags
oc get imagestreamtags -n app-dashboard

# Example output:
# NAME                                                      IMAGE REFERENCE                                                                                                                                                              UPDATED
# app-dashboard-console-plugin:amd64-0.0.1-20260512222800   image-registry.openshift-image-registry.svc:5000/app-dashboard/app-dashboard-console-plugin@sha256:02897830373e766276833ab78e9ff8121c9cc4f20e035d92f322dbb5c35413e5   24 minutes ago
```

### Check Currently Running Image

```bash
# Get the image currently running in the deployment
oc get deployment app-dashboard -n app-dashboard -o jsonpath='{.spec.template.spec.containers[0].image}'

# Example output:
# image-registry.openshift-image-registry.svc:5000/app-dashboard/app-dashboard-console-plugin:amd64-0.0.1-20260512222800
```

### Check Pod Status

```bash
# View running pods
oc get pods -n app-dashboard -l app=app-dashboard

# View pod logs
oc logs -f deployment/app-dashboard -n app-dashboard
```

## Development

### Local Development

In one terminal:
```bash
yarn install
yarn run start
```

In another terminal:
```bash
oc login
yarn run start-console
```

Navigate to <http://localhost:9000> to see the plugin.

### Running with Apple Silicon and Podman

If using podman on Mac with Apple silicon:

```bash
podman machine ssh
sudo -i
rpm-ostree install qemu-user-static
systemctl reboot
```

## Configuration Fields

### App Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `enabled` | boolean | Yes | Show/hide app in dashboard |
| `displayName` | string | Yes | Display name in dashboard |
| `category` | string | Yes | Category for grouping (Media, Development, Services, etc.) |
| `description` | string | No | App description |
| `primaryRoute` | string | No | Main route name for "Open" button |
| `customLinks` | array | No | Additional routes/links |

### Custom Link Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Link identifier |
| `route` | string | No* | OpenShift route name (auto-resolved to URL) |
| `url` | string | No* | Direct URL (use if route not available) |
| `description` | string | No | Custom text for link (default: "Additional route") |

*Either `route` or `url` must be provided

## Architecture

The plugin works with the App Dashboard Operator:

1. **Operator** watches labeled namespaces and creates/updates ConfigMaps
2. **Plugin** reads ConfigMaps and displays apps in the dashboard
3. **Operator** resolves route names to full URLs in ConfigMaps
4. **Plugin** renders apps with primary routes and custom links

## Troubleshooting

### Plugin Not Loading

```bash
# Check console operator config
oc get console.operator.openshift.io cluster -o yaml

# Verify plugin is enabled
oc get consoleplugin app-dashboard

# Restart console pods
oc delete pods -n openshift-console -l app=console
```

### Apps Not Showing

```bash
# Check namespace label
oc get namespace media --show-labels

# Check ConfigMap exists
oc get configmap dashboard-config-media -n media

# View ConfigMap content
oc get configmap dashboard-config-media -n media -o yaml
```

### Clear Browser Cache

Hard refresh your browser:
- Chrome/Firefox: `Ctrl+Shift+R` (Windows/Linux) or `Cmd+Shift+R` (Mac)
- Or use incognito/private mode

## References

- [Console Plugin SDK](https://github.com/openshift/console/tree/main/frontend/packages/console-dynamic-plugin-sdk)
- [Dynamic Plugin Enhancement Proposal](https://github.com/openshift/enhancements/blob/master/enhancements/console/dynamic-plugins.md)
- [App Dashboard Operator](https://github.com/fumbles/yamlwrangler-operator)

## License

Apache 2.0
