# Publishing OLM-Ready Images to DockerHub

## Overview

To enable OLM installation from DockerHub (so the operator appears in OpenShift's Installed Operators UI), you need to publish **4 images** to DockerHub:

1. **Operator image**: `fumbles/yamlwrangler-operator`
2. **Plugin image**: `fumbles/yamlwrangler-dashboard`
3. **Bundle image**: `fumbles/yamlwrangler-operator-bundle` (NEW)
4. **Catalog image**: `fumbles/yamlwrangler-operator-catalog` (NEW)

## Why 4 Images?

- **Operator + Plugin**: The actual workloads that run
- **Bundle**: OLM metadata package (CSV, CRDs) that references operator/plugin images
- **Catalog**: OLM catalog index that references the bundle image

## Step-by-Step Publishing

### 1. Login to DockerHub

```bash
podman login docker.io
```

### 2. Build and Push All Images

```bash
# Set version
VERSION=v1.0.1

# Build operator and plugin, push to OpenShift registry AND DockerHub
./build-and-deploy.sh ${VERSION} --ship --olm
```

This pushes to DockerHub:
- ✅ `docker.io/fumbles/yamlwrangler-operator:${VERSION}`
- ✅ `docker.io/fumbles/yamlwrangler-dashboard:${VERSION}`

But bundle/catalog are only in OpenShift internal registry.

### 3. Modify Bundle to Reference DockerHub Images

The bundle needs to reference DockerHub images instead of internal registry. Edit the generated CSV before building the bundle:

```bash
# After step 2, the bundle is in a temp directory
# We need to rebuild it with DockerHub references

VERSION=v1.0.1
DOCKERHUB_ORG=fumbles
TMP_OLM="$(mktemp -d)"
CSV_VERSION="${VERSION#v}"
CSV_NAME="app-dashboard-operator.v${CSV_VERSION}"

# Generate CSV with DockerHub images
sed \
  -e "s|name: app-dashboard-operator\.v.*|name: ${CSV_NAME}|" \
  -e "s|containerImage: .*|containerImage: docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator:${VERSION}|" \
  -e "s|image: fumbles/yamlwrangler-operator:.*|image: docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator:${VERSION}|" \
  -e "s|image: fumbles/yamlwrangler-dashboard:.*|image: docker.io/${DOCKERHUB_ORG}/yamlwrangler-dashboard:${VERSION}|" \
  manifests/olm/clusterserviceversion.yaml.template > "${TMP_OLM}/${CSV_NAME}.clusterserviceversion.yaml"

# Update version field
awk -v version="${CSV_VERSION}" '
  !replaced && /^  version:/ {
    print "  version: " version
    replaced = 1
    next
  }
  { print }
' "${TMP_OLM}/${CSV_NAME}.clusterserviceversion.yaml" > "${TMP_OLM}/${CSV_NAME}.clusterserviceversion.yaml.tmp"
mv "${TMP_OLM}/${CSV_NAME}.clusterserviceversion.yaml.tmp" "${TMP_OLM}/${CSV_NAME}.clusterserviceversion.yaml"

# Create bundle structure
mkdir -p "${TMP_OLM}/bundle/manifests" "${TMP_OLM}/bundle/metadata"
cp "${TMP_OLM}/${CSV_NAME}.clusterserviceversion.yaml" "${TMP_OLM}/bundle/manifests/"
cp manifests/crds/*.yaml "${TMP_OLM}/bundle/manifests/"

# Create bundle metadata
cat > "${TMP_OLM}/bundle/metadata/annotations.yaml" <<EOF
annotations:
  operators.operatorframework.io.bundle.mediatype.v1: registry+v1
  operators.operatorframework.io.bundle.manifests.v1: manifests/
  operators.operatorframework.io.bundle.metadata.v1: metadata/
  operators.operatorframework.io.bundle.package.v1: app-dashboard-operator
  operators.operatorframework.io.bundle.channels.v1: alpha
  operators.operatorframework.io.bundle.channel.default.v1: alpha
EOF

# Create bundle Dockerfile
cat > "${TMP_OLM}/bundle.Dockerfile" <<EOF
FROM scratch
LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=app-dashboard-operator
LABEL operators.operatorframework.io.bundle.channels.v1=alpha
LABEL operators.operatorframework.io.bundle.channel.default.v1=alpha
COPY bundle/manifests /manifests
COPY bundle/metadata /metadata
EOF

# Build and push bundle to DockerHub
podman build --platform linux/amd64 \
  -f "${TMP_OLM}/bundle.Dockerfile" \
  -t "docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator-bundle:${VERSION}" \
  "${TMP_OLM}"
podman push "docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator-bundle:${VERSION}"

echo "✓ Bundle pushed to docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator-bundle:${VERSION}"
```

### 4. Build and Push Catalog Image

```bash
VERSION=v1.0.1
DOCKERHUB_ORG=fumbles
CSV_VERSION="${VERSION#v}"
CSV_NAME="app-dashboard-operator.v${CSV_VERSION}"
TMP_CATALOG="$(mktemp -d)"

# Create catalog structure
mkdir -p "${TMP_CATALOG}/catalog"

cat > "${TMP_CATALOG}/catalog/catalog.yaml" <<EOF
schema: olm.package
name: app-dashboard-operator
defaultChannel: alpha
---
schema: olm.channel
package: app-dashboard-operator
name: alpha
entries:
  - name: ${CSV_NAME}
---
schema: olm.bundle
name: ${CSV_NAME}
package: app-dashboard-operator
image: docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator-bundle:${VERSION}
properties:
  - type: olm.package
    value:
      packageName: app-dashboard-operator
      version: ${CSV_VERSION}
  - type: olm.gvk
    value:
      group: dashboard.yamlwrangler.com
      kind: AppDashboard
      version: v1alpha1
  - type: olm.gvk
    value:
      group: dashboard.yamlwrangler.com
      kind: DashboardAppGroup
      version: v1alpha1
  - type: olm.gvk
    value:
      group: dashboard.yamlwrangler.com
      kind: DashboardNamespaceConfig
      version: v1alpha1
  - type: olm.gvk
    value:
      group: dashboard.yamlwrangler.com
      kind: DashboardLink
      version: v1alpha1
relatedImages:
  - name: operator
    image: docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator:${VERSION}
  - name: dashboard
    image: docker.io/${DOCKERHUB_ORG}/yamlwrangler-dashboard:${VERSION}
EOF

# Create catalog Dockerfile
cat > "${TMP_CATALOG}/catalog.Dockerfile" <<EOF
FROM quay.io/operator-framework/opm:latest
COPY catalog /configs
ENTRYPOINT ["/bin/opm"]
CMD ["serve", "/configs"]
EOF

# Build and push catalog to DockerHub
podman build --platform linux/amd64 \
  -f "${TMP_CATALOG}/catalog.Dockerfile" \
  -t "docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator-catalog:${VERSION}" \
  "${TMP_CATALOG}"
podman push "docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator-catalog:${VERSION}"

echo "✓ Catalog pushed to docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator-catalog:${VERSION}"

# Cleanup
rm -rf "${TMP_OLM}" "${TMP_CATALOG}"
```

### 5. Tag as Latest (Optional)

```bash
VERSION=v1.0.1
DOCKERHUB_ORG=fumbles

# Tag all images as latest
podman tag "docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator:${VERSION}" \
  "docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator:latest"
podman tag "docker.io/${DOCKERHUB_ORG}/yamlwrangler-dashboard:${VERSION}" \
  "docker.io/${DOCKERHUB_ORG}/yamlwrangler-dashboard:latest"
podman tag "docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator-bundle:${VERSION}" \
  "docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator-bundle:latest"
podman tag "docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator-catalog:${VERSION}" \
  "docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator-catalog:latest"

# Push latest tags
podman push "docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator:latest"
podman push "docker.io/${DOCKERHUB_ORG}/yamlwrangler-dashboard:latest"
podman push "docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator-bundle:latest"
podman push "docker.io/${DOCKERHUB_ORG}/yamlwrangler-operator-catalog:latest"
```

## User Installation from DockerHub

Once published, users install via OLM with:

### 1. Create CatalogSource

```bash
kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: yamlwrangler-catalog
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: docker.io/fumbles/yamlwrangler-operator-catalog:v1.0.1
  displayName: Yamlwrangler Operators
  publisher: Yamlwrangler
  updateStrategy:
    registryPoll:
      interval: 10m
EOF
```

### 2. Wait for CatalogSource to be Ready

```bash
kubectl get catalogsource yamlwrangler-catalog -n openshift-marketplace -w
```

### 3. Install from OpenShift Console

1. Navigate to **Operators → OperatorHub**
2. Search for "Yamlwrangler"
3. Click **Install**
4. Choose namespace: `app-dashboard-operator`
5. Click **Install**

### 4. Verify Installation

```bash
# Check operator appears in Installed Operators
oc get operator -n app-dashboard-operator

# Check CSV
oc get csv -n app-dashboard-operator

# Check pods
oc get pods -n app-dashboard-operator
```

### 5. Create AppDashboard Instance

```bash
kubectl apply -f - <<EOF
apiVersion: dashboard.yamlwrangler.com/v1alpha1
kind: AppDashboard
metadata:
  name: yamlwrangler
spec:
  namespace: app-dashboard
  pluginName: app-dashboard
  displayName: Yamlwrangler App Dashboard
  image: docker.io/fumbles/yamlwrangler-dashboard:v1.0.1
  replicas: 2
  enableConsolePlugin: true
EOF
```

## Summary

**Yes, you need 4 DockerHub repositories:**

1. `fumbles/yamlwrangler-operator` - Operator runtime
2. `fumbles/yamlwrangler-dashboard` - Console plugin runtime
3. `fumbles/yamlwrangler-operator-bundle` - OLM bundle (metadata)
4. `fumbles/yamlwrangler-operator-catalog` - OLM catalog (index)

The bundle and catalog are small metadata-only images that reference the actual runtime images.

## Automation Script

Consider creating a `publish-to-dockerhub.sh` script that automates steps 1-5 above for easier releases.