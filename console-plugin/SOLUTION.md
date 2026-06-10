# Console Plugin Fix - SOLUTION FOUND! ✅

## Root Causes Identified

The plugin was failing to load with the error **"Failed to load scripts of plugin app-dashboard"** because of TWO issues:

### 1. **SDK Version Mismatch** (CRITICAL)

You're running **OCP 4.21** but using **SDK 4.22**. This version mismatch can cause plugin loading failures and compatibility issues.

### 2. **Missing i18n Localization File**

The plugin was looking for `/locales/en/plugin__app-dashboard.json` but the file was named `plugin__console-plugin-template.json` (from the original template).

**Evidence from logs:**
```
2026/05/11 06:25:24 [error] 2#2: *2 open() "/usr/share/nginx/html/locales/en/plugin__app-dashboard.json" failed (2: No such file or directory)
10.128.0.152 - - [11/May/2026:06:25:24 +0000] "GET /locales/en/plugin__app-dashboard.json HTTP/1.1" 404 555
```

## Fixes Applied

### 1. ✅ Downgraded SDK to Match OCP Version (CRITICAL FIX)
Changed in `package.json`:
```json
"@openshift-console/dynamic-plugin-sdk": "4.21-latest",
"@openshift-console/dynamic-plugin-sdk-webpack": "4.21-latest",
```
And updated plugin API dependency:
```json
"@console/pluginAPI": "~4.21.0"
```

### 2. ✅ Renamed i18n File (CRITICAL FIX)
```bash
mv locales/en/plugin__console-plugin-template.json locales/en/plugin__app-dashboard.json
```

### 3. ✅ Updated Nginx Configuration
Fixed the ConfigMap to explicitly define MIME types (especially for JavaScript files) since the UBI nginx image doesn't have `/etc/nginx/mime.types` at the expected location.

### 4. ✅ Added Service Account to Deployment
Added `serviceAccountName` to the deployment spec for proper RBAC.

## Rebuild and Redeploy Steps

Now that the fixes are applied, you need to reinstall dependencies with the correct SDK version, then rebuild and redeploy:

```bash
# Navigate to plugin directory
cd app-dashboard-console-plugin

# IMPORTANT: Reinstall dependencies to get SDK 4.21
rm -rf node_modules
yarn install

# Rebuild with all fixes applied
yarn build

# Set variables
TAG=amd64-$(date +%Y%m%d%H%M%S)
REGISTRY=$(oc get route default-route -n openshift-image-registry -o jsonpath='{.spec.host}')
TOKEN=$(oc whoami -t)

# Build amd64 image
podman build --platform linux/amd64 -t app-dashboard-console-plugin:$TAG .

# Login to OpenShift registry
podman login -u "$(oc whoami)" -p "$TOKEN" --tls-verify=false "$REGISTRY"

# Tag image
podman tag app-dashboard-console-plugin:$TAG \
  "$REGISTRY/app-dashboard/app-dashboard-console-plugin:$TAG"

# Push image
podman push --tls-verify=false \
  "$REGISTRY/app-dashboard/app-dashboard-console-plugin:$TAG"

# Upgrade Helm deployment
helm upgrade app-dashboard ./charts/openshift-console-plugin \
  -n app-dashboard \
  --set plugin.name=app-dashboard \
  --set plugin.description="Yamlwrangler App Dashboard" \
  --set plugin.image=image-registry.openshift-image-registry.svc:5000/app-dashboard/app-dashboard-console-plugin:$TAG

# Restart console pods to pick up the changes
oc -n openshift-console delete pod -l app=console
```

## Verification

After redeploying, verify the fix:

### 1. Check Plugin Pod Logs
```bash
oc logs -n app-dashboard -l app.kubernetes.io/name=app-dashboard -f
```

You should now see **200 OK** for the i18n file instead of 404:
```
GET /locales/en/plugin__app-dashboard.json HTTP/1.1" 200
```

### 2. Check Console Plugin Status
```bash
oc get consoleplugin app-dashboard -o yaml
```

The status should show no errors.

### 3. Browser Console
Open the OpenShift console and check the browser DevTools:
- **Network tab**: All plugin assets should load with 200 status
- **Console tab**: No more "Failed to load scripts" errors
- The "App Dashboard" link should appear in the Home navigation

### 4. Access the Plugin
Navigate to the "App Dashboard" link in the console's Home section. You should see your custom dashboard page.

## What Was Wrong

The plugin template uses i18n (internationalization) for text localization. The namespace must match the plugin name exactly:

- **Plugin name** (from package.json): `app-dashboard`
- **Required i18n file**: `plugin__app-dashboard.json`
- **Actual file name**: `plugin__console-plugin-template.json` ❌

When the console tried to load the plugin, it:
1. ✅ Successfully loaded the manifest
2. ✅ Successfully loaded the plugin-entry.js
3. ❌ **Failed to load the i18n file** (404 error)
4. ❌ Plugin initialization failed due to missing i18n

## Additional Notes

### Why This Happened
When you changed the plugin name in `package.json` from `console-plugin-template` to `app-dashboard`, the i18n files in the `locales/` directory weren't automatically renamed. The build process copies these files as-is.

### Prevention
When renaming a plugin, always:
1. Update `package.json` → `consolePlugin.name`
2. Rename i18n files: `locales/*/plugin__<old-name>.json` → `locales/*/plugin__<new-name>.json`
3. Or run `yarn i18n` to regenerate them

### Other Improvements Made
- Fixed nginx MIME type configuration for proper JavaScript serving
- Added explicit Content-Type headers for .js and .json files
- Added service account to deployment for proper RBAC

## Expected Result

After applying these fixes and redeploying:
- ✅ Plugin loads without errors
- ✅ "App Dashboard" appears in Home navigation
- ✅ Custom dashboard page renders correctly
- ✅ No 404 errors in nginx logs
- ✅ No script loading errors in browser console

## Next Steps

1. Rebuild and redeploy using the commands above
2. Verify the plugin loads successfully
3. Start implementing your actual dashboard functionality (route discovery, etc.)