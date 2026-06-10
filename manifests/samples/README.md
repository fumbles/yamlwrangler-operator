# Sample Custom Resources

This directory contains example Custom Resources for the Yamlwrangler Dashboard Operator.

## AppDashboard

The `AppDashboard` CR installs the console plugin into your OpenShift cluster.

**File:** [`appdashboard.yaml`](./appdashboard.yaml)

```bash
kubectl apply -f manifests/samples/appdashboard.yaml
```

This creates:
- A namespace for the plugin (`app-dashboard-plugin`)
- A deployment running the console plugin
- A service and ConfigMap
- A ConsolePlugin resource that registers with OpenShift Console
- Optionally, a ConsoleLink for navigation

## DashboardAppGroup

The `DashboardAppGroup` CR enables namespace discovery and automatically labels deployments for the dashboard.

**File:** [`dashboardappgroup.yaml`](./dashboardappgroup.yaml)

```bash
kubectl apply -f manifests/samples/dashboardappgroup.yaml
```

### What it does:

1. **Labels the namespace** with `dashboard.yamlwrangler.com/enabled=true`
2. **Creates a ConfigMap** (`dashboard-config-<namespace>`) if it doesn't exist
3. **Discovers deployments** in the namespace and populates the ConfigMap
4. **Labels matched deployments** with `dashboard.yamlwrangler.com/enabled=true`
5. **Applies metadata** (category, description, custom links) to deployments

### Use Cases:

- **UI-driven namespace enablement**: Create through OpenShift Console instead of manually labeling namespaces
- **Automatic deployment discovery**: Finds deployments matching patterns or labels
- **Metadata management**: Apply consistent categories and descriptions to groups of apps
- **Custom links**: Add additional links (docs, admin panels, etc.) to app cards

### Example: Media Apps

```yaml
apiVersion: dashboard.yamlwrangler.com/v1alpha1
kind: DashboardAppGroup
metadata:
  name: media-apps
  namespace: media
spec:
  displayName: Media Applications
  category: Media
  description: Media management and streaming applications
  autoLabel: true
  selector:
    matchPattern: "^(plex|sonarr|radarr|prowlarr).*"
  primaryRoute: plex
  customLinks:
    - name: Plex Admin
      url: https://app.plex.tv/desktop
      icon: ExternalLinkAltIcon
```

### Selector Options:

You can match deployments using:

1. **Regex pattern:**
   ```yaml
   selector:
     matchPattern: "^my-app-.*"
   ```

2. **Explicit names:**
   ```yaml
   selector:
     matchNames:
       - app1-deployment
       - app2-deployment
   ```

3. **Label selectors:**
   ```yaml
   selector:
     matchLabels:
       app.kubernetes.io/part-of: my-stack
   ```

### Status:

Check the status to see matched deployments:

```bash
kubectl get dashboardappgroup -n <namespace> -o yaml
```

The status shows:
- List of matched deployment names
- Last update timestamp
- Conditions (Ready/Error)

## DashboardNamespaceConfig

The `DashboardNamespaceConfig` CR is the structured API for creating and editing
`dashboard-config-<namespace>` ConfigMaps from the OpenShift Operator UI.

**File:** [`dashboardnamespaceconfig.yaml`](./dashboardnamespaceconfig.yaml)

```bash
kubectl apply -f manifests/samples/dashboardnamespaceconfig.yaml
```

### Discovery Modes

- `Merge`: keep existing ConfigMap entries, append discovered deployments, and overlay the CR fields.
- `Replace`: rebuild from discovered deployments and then overlay the CR fields.
- `None`: only write the apps declared in the CR, preserving existing ConfigMap content.

Use this when you want to generate the sample namespace ConfigMap, edit app metadata,
set categories, assign primary routes, or manage custom links with a typed resource.

Existing `dashboard-config-<namespace>` ConfigMaps are imported automatically as
`DashboardNamespaceConfig` operands named after the ConfigMap. Imported operands
carry the `dashboard.yamlwrangler.com/imported-from-configmap` annotation.

Deployments already labeled `dashboard.yamlwrangler.com/enabled=true` are also
backfilled into `DashboardNamespaceConfig` operands using their dashboard
annotations.

## DashboardLink

The `DashboardLink` CR is the lightweight API for adding or editing one custom
link without editing the full namespace config.

**File:** [`dashboardlink.yaml`](./dashboardlink.yaml)

```bash
kubectl apply -f manifests/samples/dashboardlink.yaml
```

The operator merges the link into the app entry in `dashboard-config-<namespace>`.
Use `spec.url` for an external URL or `spec.route` to resolve an OpenShift Route
from the same namespace.

Custom links already present in an imported ConfigMap are backfilled as
`DashboardLink` operands. Removing an imported `DashboardLink` removes that link
from the ConfigMap.

Standalone custom-link ConfigMaps labeled
`dashboard.yamlwrangler.com/type=custom-link` are imported as `DashboardLink`
operands and keep their `name`, `category`, `url`, and `description` fields in
sync.
