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