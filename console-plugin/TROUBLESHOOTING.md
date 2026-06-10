# Console Plugin Troubleshooting Guide

## Identified Issues

### 1. **CRITICAL: Nginx Configuration Missing MIME Types**

The nginx.conf in the ConfigMap is missing proper MIME type handling for JavaScript files. This will cause the browser to reject `.js` files with incorrect Content-Type headers.

**Problem**: The nginx config includes `/etc/nginx/mime.types` but this file doesn't exist in the UBI nginx image at that location.

**Symptoms**:
- Browser console errors like "Failed to load module script" or "MIME type mismatch"
- Scripts loaded with `text/plain` instead of `application/javascript`
- Plugin fails to initialize

### 2. **Plugin Manifest baseURL Mismatch**

The plugin manifest shows `"baseURL":"/api/plugins/app-dashboard/"` but the nginx configuration serves from root `/`. This causes 404 errors when the console tries to load plugin assets.

**Symptoms**:
- 404 errors for plugin-entry.js and chunk files
- Console errors about failed script loading
- Plugin appears in console but doesn't load

### 3. **Missing Service Account in Deployment**

The deployment template doesn't specify a serviceAccount, which may cause permission issues.

### 4. **Port Mismatch in Nginx**

The Dockerfile exposes port 80, but the helm chart configures nginx to listen on port 9443 (SSL). The container needs to match.

## Solutions

### Fix 1: Update Nginx Configuration

The nginx config needs to explicitly define MIME types and serve from the correct base path.

### Fix 2: Update Webpack Configuration

The webpack config needs to set the correct publicPath to match the plugin's baseURL.

### Fix 3: Add Service Account to Deployment

### Fix 4: Align Ports

Either update Dockerfile to use 9443 or update helm values to use 80 (non-SSL).

## Recommended Approach

For OpenShift console plugins, it's better to:
1. Use HTTP (port 9001) internally - OpenShift handles TLS at the service level
2. Serve from root path with proper MIME types
3. Let the ConsolePlugin resource handle the routing

See fixes below.