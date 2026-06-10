# Usage

## Install The Dashboard

```yaml
apiVersion: dashboard.yamlwrangler.com/v1alpha1
kind: AppDashboard
metadata:
  name: yamlwrangler
spec:
  namespace: app-dashboard
  pluginName: app-dashboard
  displayName: Yamlwrangler App Dashboard
  image: image-registry.openshift-image-registry.svc:5000/app-dashboard/app-dashboard-console-plugin:v1.0.0
  replicas: 2
  enableConsolePlugin: true
```

## Generate Or Edit Namespace Config

Create a `DashboardNamespaceConfig` in the namespace you want to manage:

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
      primaryRoute: plex
      customLinks:
        - name: Admin
          route: plex
```

The operator writes `dashboard-config-media` and the ConfigMap controller applies
labels/annotations to deployments.

Existing `dashboard-config-<namespace>` ConfigMaps are imported automatically.
The ConfigMap controller creates a `DashboardNamespaceConfig` named after the
ConfigMap and marks it with
`dashboard.yamlwrangler.com/imported-from-configmap`, so existing namespace
config appears in the Installed Operator UI without manual conversion.

If no namespace ConfigMap exists yet, the namespace controller can also backfill
`DashboardNamespaceConfig` from deployments already labeled
`dashboard.yamlwrangler.com/enabled=true` and their dashboard annotations.

## Add One Link

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
```

Use `spec.route` instead of `spec.url` to resolve a Route in the same namespace.
Custom links already present in imported ConfigMaps are exposed as
`DashboardLink` operands. Deleting an imported `DashboardLink` removes that link
from the backing ConfigMap.

Standalone custom-link ConfigMaps labeled
`dashboard.yamlwrangler.com/type=custom-link` are also imported as
`DashboardLink` operands. For those imports, edits to `name`, `category`, `url`,
and `description` sync back to the source ConfigMap.

## Group Apps

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
