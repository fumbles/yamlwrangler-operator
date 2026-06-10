# App Dashboard - Complete How-To Guide

This guide covers everything you need to know about the App Dashboard OpenShift Console Plugin and its companion Operator.

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Quick Start](#quick-start)
4. [Working with Namespaces](#working-with-namespaces)
5. [ConfigMap Configuration](#configmap-configuration)
6. [Custom Links](#custom-links)
7. [Building and Deploying](#building-and-deploying)
8. [Troubleshooting](#troubleshooting)

---

## Overview

The App Dashboard is a two-part system for managing and displaying applications in OpenShift:

1. **Console Plugin** - A React-based UI that displays applications in the OpenShift web console
2. **Operator** - A Kubernetes operator that automatically discovers deployments and manages ConfigMaps

### Why Two Components?

- **Console Plugin**: Provides the user interface and visualization
- **Operator**: Automates discovery and ConfigMap generation, resolves routes to URLs

---

## Architecture

### How It Works

```
┌─────────────────────┐
│  Label Namespace    │  ← You label a namespace
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Namespace Controller│  ← Discovers all deployments
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  ConfigMap Created  │  ← Auto-generated with all deployments
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ ConfigMap Controller│  ← Resolves route names to URLs
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Console Plugin     │  ← Reads ConfigMaps and displays apps
└─────────────────────┘
```

### Key Concepts

- **Namespace Discovery**: Label a namespace to enable auto-discovery
- **ConfigMap-driven**: Single ConfigMap per namespace contains all app settings
- **Route Resolution**: Operator automatically resolves OpenShift route names to full URLs
- **Custom Links**: Support for multiple routes per deployment (sidecars, additional services)

---

## Quick Start

### Prerequisites

- OpenShift cluster with admin access
- `kubectl` or `oc` CLI configured
- Node.js 18+ (for building the plugin)
- Go 1.21+ (for building the operator)
- Podman or Docker

### 1. Deploy the Operator

```bash
cd app-dashboard-operator

# Build and deploy the operator
./build-and-deploy.sh

# Verify it's running
kubectl get pods -n app-dashboard-operator
```

### 2. Deploy the Console Plugin

```bash
cd app-dashboard-console-plugin

# Build and deploy the plugin
npm run build && ./build-tag.sh

# Verify it's running
kubectl get pods -n app-dashboard
```

### 3. Add Application Menu Link

```bash
cd app-dashboard-console-plugin

# Apply the console link
oc apply -f console-link.yaml
```

This adds the dashboard to the OpenShift Application Menu.

### 4. Enable Discovery for a Namespace

```bash
# Label a namespace to enable auto-discovery
oc label namespace media dashboard.yamlwrangler.com/enabled=true

# Check that ConfigMap was created
oc get configmap dashboard-config-media -n media
```

### 5. Customize App Settings

```bash
# Edit the generated ConfigMap
oc edit configmap dashboard-config-media -n media
```

### 6. View in Dashboard

1. Open OpenShift Console
2. Click the Application Menu (9 dots icon)
3. Select "App Dashboard" under "Red Hat Applications"
4. Your apps should now be visible!

---

## Working with Namespaces

### Enabling Discovery

Label any namespace to enable automatic discovery:

```bash
# Enable discovery
oc label namespace <namespace> dashboard.yamlwrangler.com/enabled=true

# Disable discovery
oc label namespace <namespace> dashboard.yamlwrangler.com/enabled-
```

### What Happens When You Label a Namespace?

1. **Operator detects** the labeled namespace
2. **Discovers all deployments** in that namespace
3. **Creates ConfigMap** named `dashboard-config-<namespace>`
4. **Populates ConfigMap** with templates for each deployment
5. **You customize** the ConfigMap to set display names, categories, etc.

### Checking Discovery Status

```bash
# List all labeled namespaces
oc get namespaces -l dashboard.yamlwrangler.com/enabled=true

# Check if ConfigMap was created
oc get configmap -n <namespace> | grep dashboard-config

# View ConfigMap content
oc get configmap dashboard-config-<namespace> -n <namespace> -o yaml
```

---

## ConfigMap Configuration

### ConfigMap Structure

Each namespace gets one ConfigMap named `dashboard-config-<namespace>`:

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
    
    sonarr:
      enabled: true
      displayName: Sonarr
      category: Media
      description: TV show management
      primaryRoute: sonarr
```

### Configuration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `enabled` | boolean | Yes | Show/hide app in dashboard |
| `displayName` | string | Yes | Display name in dashboard |
| `category` | string | Yes | Category for grouping |
| `description` | string | No | App description |
| `primaryRoute` | string | No | Main route name for "Open" button |
| `customLinks` | array | No | Additional routes/links |

### Standard Categories

- `Media` - Media servers, downloaders
- `Services` - General services
- `Development` - Dev tools, IDEs
- `Infrastructure` - System components
- `Monitoring` - Monitoring and observability
- `AI / Experimental` - AI and experimental tools
- `Games` - Gaming servers
- `Links` - External links and bookmarks

### Editing ConfigMaps

```bash
# Edit in your editor
oc edit configmap dashboard-config-media -n media

# Or export, edit, and apply
oc get configmap dashboard-config-media -n media -o yaml > media-config.yaml
# Edit media-config.yaml
oc apply -f media-config.yaml
```

---

## Custom Links

### What Are Custom Links?

Custom links allow you to show multiple routes for a single deployment. This is useful for:
- Deployments with sidecars (e.g., vpn-firefox with vpn-metube)
- Apps with multiple interfaces
- Additional services or admin panels

### Route-Based Custom Links (Recommended)

The operator automatically resolves route names to full URLs:

```yaml
vpn-firefox:
  enabled: true
  displayName: vpn-Firefox
  category: Media
  description: vpn-backed Firefox
  primaryRoute: vpn-firefox
  customLinks:
    - name: vpn-metube
      route: vpn-metube              # Route name
      description: Metube sidecar    # Optional description
```

**What the operator does:**
1. Finds the route named `vpn-metube`
2. Resolves it to full URL: `https://vpn-metube-media.apps.sno.yamlwrangler.com`
3. Updates the ConfigMap with the resolved URL

### URL-Based Custom Links

For external services or manual URLs:

```yaml
my-app:
  enabled: true
  displayName: My App
  category: Services
  primaryRoute: my-app
  customLinks:
    - name: Documentation
      url: https://docs.example.com
      description: User documentation
    - name: Admin Panel
      url: https://admin.example.com
      description: Admin interface
```

### Standalone Custom Links

For links not tied to a deployment, use the `add-custom-link.sh` script:

```bash
# Add an external link
./add-custom-link.sh media wyze-cameras "Wyze Cameras" "Links" "Wyze camera streams" "https://my.wyze.com/live"

# Add another external service
./add-custom-link.sh monitoring grafana-cloud "Grafana Cloud" "Monitoring" "Cloud Grafana" "https://grafana.example.com"
```

This creates a separate ConfigMap that the dashboard picks up automatically.

---

## Building and Deploying

### When to Rebuild

#### Rebuild the Operator When:
- You modify the operator code (`controllers/`, `main.go`)
- You change how ConfigMaps are generated
- You add new features to route resolution

#### Rebuild the Plugin When:
- You modify the UI code (`src/components/`)
- You change how apps are displayed
- You add new dashboard features

### Building the Operator

```bash
cd app-dashboard-operator

# Quick build and deploy (auto-generates timestamp tag)
./build-and-deploy.sh

# Check running version
oc get imagestreamtags -n app-dashboard-operator
oc get deployment app-dashboard-operator -n app-dashboard-operator -o jsonpath='{.spec.template.spec.containers[0].image}'
```

**The script automatically:**
1. Builds the Go binary
2. Creates container image with timestamp tag (e.g., `v1.0.0-20260513040330`)
3. Pushes to OpenShift internal registry
4. Deploys the operator
5. Waits for rollout to complete

### Building the Console Plugin

```bash
cd app-dashboard-console-plugin

# Quick build and deploy (auto-generates timestamp tag)
npm run build && ./build-tag.sh

# Check running version
oc get imagestreamtags -n app-dashboard
oc get deployment app-dashboard -n app-dashboard -o jsonpath='{.spec.template.spec.containers[0].image}'
```

**The script automatically:**
1. Builds the React app with `yarn build`
2. Creates Docker image with timestamp tag (e.g., `amd64-0.0.1-20260513040330`)
3. Pushes to OpenShift internal registry
4. Updates Helm deployment with new image
5. Restarts console pods

### Verifying Deployments

```bash
# Check operator
oc get pods -n app-dashboard-operator
oc logs -f deployment/app-dashboard-operator -n app-dashboard-operator

# Check plugin
oc get pods -n app-dashboard
oc get consoleplugin app-dashboard

# Check ConfigMaps
oc get configmap -n <namespace> | grep dashboard-config
```

---

## Troubleshooting

### Apps Not Showing in Dashboard

**Check 1: Is the namespace labeled?**

```bash
oc get namespace <namespace> --show-labels | grep dashboard
```

Should show: `dashboard.yamlwrangler.com/enabled=true`

**Check 2: Does the ConfigMap exist?**

```bash
oc get configmap dashboard-config-<namespace> -n <namespace>
```

**Check 3: Is the app enabled in ConfigMap?**

```bash
oc get configmap dashboard-config-<namespace> -n <namespace> -o yaml | grep -A 5 "my-app:"
```

Check that `enabled: true`

**Check 4: Are there errors in the operator?**

```bash
oc logs -f deployment/app-dashboard-operator -n app-dashboard-operator | grep ERROR
```

**Check 5: Is the plugin running?**

```bash
oc get pods -n app-dashboard
oc get consoleplugin app-dashboard
```

**Check 6: Hard refresh the browser**

Press `Ctrl+Shift+R` (Windows/Linux) or `Cmd+Shift+R` (Mac) to clear the cache.

### ConfigMap Not Created

**Check namespace label:**

```bash
oc get namespace <namespace> --show-labels
```

**Check operator logs:**

```bash
oc logs deployment/app-dashboard-operator -n app-dashboard-operator | grep <namespace>
```

**Manually trigger by re-labeling:**

```bash
oc label namespace <namespace> dashboard.yamlwrangler.com/enabled=true --overwrite
```

### Route URLs Not Showing

**Check 1: Does the route exist?**

```bash
oc get route -n <namespace>
```

**Check 2: Does the route name match?**

The `primaryRoute` in your ConfigMap should match the route name exactly.

**Check 3: Check operator logs for route resolution:**

```bash
oc logs deployment/app-dashboard-operator -n app-dashboard-operator | grep "route"
```

### Custom Links Not Resolving

**Check 1: Is the route field correct?**

```yaml
customLinks:
  - name: my-service
    route: my-service  # Must match exact route name
```

**Check 2: Check operator logs:**

```bash
oc logs deployment/app-dashboard-operator -n app-dashboard-operator | grep "customLinks"
```

**Check 3: Verify route exists:**

```bash
oc get route <route-name> -n <namespace>
```

### Dashboard Shows Wrong Category

Edit the ConfigMap and change the category:

```bash
oc edit configmap dashboard-config-<namespace> -n <namespace>
```

Change the `category` field and save. The dashboard will update automatically.

---

## Advanced Topics

### Labeling Routes for Better Matching

Label routes to associate them with deployments:

```bash
# Label a single route
oc label route plex dashboard.yamlwrangler.com/deployment=plex -n media

# Use the script to label all routes in a namespace
cd app-dashboard-console-plugin
./label-routes.sh media
```

### Monitoring the Operator

```bash
# Watch operator logs in real-time
oc logs -f deployment/app-dashboard-operator -n app-dashboard-operator

# Check operator status
oc get deployment -n app-dashboard-operator

# Check for recent events
oc get events -n app-dashboard-operator --sort-by='.lastTimestamp'
```

### Checking Image Versions

```bash
# Plugin images
oc get imagestreamtags -n app-dashboard

# Operator images
oc get imagestreamtags -n app-dashboard-operator

# Currently running versions
echo "Operator:" && oc get deployment app-dashboard-operator -n app-dashboard-operator -o jsonpath='{.spec.template.spec.containers[0].image}'
echo "Plugin:" && oc get deployment app-dashboard -n app-dashboard -o jsonpath='{.spec.template.spec.containers[0].image}'
```

---

## Best Practices

1. **Organize by namespace** - Use namespaces to group related applications
2. **Use consistent categories** - Stick to a standard set of categories across all namespaces
3. **Document custom links** - Add helpful descriptions for each custom link
4. **Use route-based links** - Let the operator resolve routes instead of hardcoding URLs
5. **Keep ConfigMaps in version control** - Export and commit your ConfigMaps to Git
6. **Monitor operator logs** - Watch for errors when making changes
7. **Test in dev first** - Try changes in a dev namespace before production

---

## Reference

### Operator Files

- `controllers/namespace_controller.go` - Namespace watch & ConfigMap generation
- `controllers/configmap_controller.go` - ConfigMap watch & route resolution
- `main.go` - Operator entry point
- `manifests/deploy/` - Operator deployment manifests
- `build-and-deploy.sh` - Build and deploy script

### Plugin Files

- `src/components/AppDashboardPage.tsx` - Main dashboard component
- `charts/openshift-console-plugin/` - Helm chart
- `build-tag.sh` - Build and deploy script
- `add-custom-link.sh` - Script to add standalone custom links
- `label-routes.sh` - Script to label routes
- `console-link.yaml` - Application menu link definition

### Useful Commands

```bash
# List all labeled namespaces
oc get namespaces -l dashboard.yamlwrangler.com/enabled=true

# List all dashboard ConfigMaps
oc get configmaps --all-namespaces | grep dashboard-config

# View a ConfigMap
oc get configmap dashboard-config-<namespace> -n <namespace> -o yaml

# Edit a ConfigMap
oc edit configmap dashboard-config-<namespace> -n <namespace>

# Restart operator
oc rollout restart deployment/app-dashboard-operator -n app-dashboard-operator

# Restart plugin
oc rollout restart deployment/app-dashboard -n app-dashboard

# Check console plugin status
oc get consoleplugin app-dashboard
```

---

## Getting Help

- Check the operator logs for errors
- Verify the namespace is labeled correctly
- Make sure the ConfigMap exists and apps are enabled
- Hard refresh your browser to clear the console cache
- Check that both operator and plugin pods are running
- Verify routes exist and names match exactly

---

## Contributing

When making changes:

1. **Operator changes**: Modify Go code, rebuild operator, test with multiple namespaces
2. **Plugin changes**: Modify React code, rebuild plugin, test in console
3. **Document changes**: Update this guide and README files
4. **Test thoroughly**: Verify with multiple apps and scenarios

---

## License

Apache 2.0