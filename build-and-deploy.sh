#!/bin/bash

set -e

# Configuration
REGISTRY="${REGISTRY:-default-route-openshift-image-registry.apps.sno.yamlwrangler.com}"
INTERNAL_REGISTRY="${INTERNAL_REGISTRY:-image-registry.openshift-image-registry.svc:5000}"
OPERATOR_NAMESPACE="${OPERATOR_NAMESPACE:-app-dashboard-operator}"
PLUGIN_NAMESPACE="${PLUGIN_NAMESPACE:-app-dashboard}"
OPERATOR_IMAGE_NAME="${OPERATOR_IMAGE_NAME:-app-dashboard-operator}"
PLUGIN_IMAGE_NAME="${PLUGIN_IMAGE_NAME:-app-dashboard-console-plugin}"
BUNDLE_IMAGE_NAME="${BUNDLE_IMAGE_NAME:-app-dashboard-operator-bundle}"
CATALOG_IMAGE_NAME="${CATALOG_IMAGE_NAME:-app-dashboard-operator-catalog}"
APP_DASHBOARD_NAME="${APP_DASHBOARD_NAME:-yamlwrangler}"
DOCKERHUB_ORG="${DOCKERHUB_ORG:-fumbles}"
DOCKERHUB_OPERATOR_IMAGE_NAME="${DOCKERHUB_OPERATOR_IMAGE_NAME:-yamlwrangler-operator}"
DOCKERHUB_PLUGIN_IMAGE_NAME="${DOCKERHUB_PLUGIN_IMAGE_NAME:-yamlwrangler-dashboard}"
PACKAGE_NAME="${PACKAGE_NAME:-app-dashboard-operator}"
CHANNEL="${CHANNEL:-alpha}"
SHIP=false
OLM=false
TAG=""

usage() {
  cat <<EOF
Usage: $0 [tag] [--ship] [--olm]

Arguments:
  tag       Optional image tag. Defaults to v1.0.0-<timestamp>.

Flags:
  --ship   Also tag and push operator/plugin images to Docker Hub.
  --olm    Install the operator through OLM CSV/OperatorGroup so oc get operator reports it.

Environment overrides:
  REGISTRY                         ${REGISTRY}
  INTERNAL_REGISTRY                ${INTERNAL_REGISTRY}
  OPERATOR_NAMESPACE               ${OPERATOR_NAMESPACE}
  PLUGIN_NAMESPACE                 ${PLUGIN_NAMESPACE}
  DOCKERHUB_ORG                    ${DOCKERHUB_ORG}
  DOCKERHUB_OPERATOR_IMAGE_NAME    ${DOCKERHUB_OPERATOR_IMAGE_NAME}
  DOCKERHUB_PLUGIN_IMAGE_NAME      ${DOCKERHUB_PLUGIN_IMAGE_NAME}
  BUNDLE_IMAGE_NAME                ${BUNDLE_IMAGE_NAME}
  CATALOG_IMAGE_NAME               ${CATALOG_IMAGE_NAME}
  PACKAGE_NAME                     ${PACKAGE_NAME}
  CHANNEL                          ${CHANNEL}
EOF
}

wait_for_catalogsource() {
  local name="$1"
  local namespace="$2"
  local state=""

  for _ in {1..60}; do
    state="$(kubectl get catalogsource "${name}" -n "${namespace}" -o jsonpath='{.status.connectionState.lastObservedState}' 2>/dev/null || true)"
    if [ "${state}" = "READY" ]; then
      echo "✓ CatalogSource ${name} is READY"
      return 0
    fi
    if [ "${state}" = "TRANSIENT_FAILURE" ]; then
      echo "CatalogSource ${name} is transiently unavailable; waiting for OLM to reconnect..."
    fi
    sleep 5
  done

  echo "✗ CatalogSource ${name} did not become READY"
  kubectl get catalogsource "${name}" -n "${namespace}" -o yaml || true
  return 1
}

wait_for_subscription_csv() {
  local subscription="$1"
  local namespace="$2"
  local csv_name=""
  local phase=""

  for _ in {1..90}; do
    csv_name="$(kubectl get subscription "${subscription}" -n "${namespace}" -o jsonpath='{.status.installedCSV}' 2>/dev/null || true)"
    if [ -n "${csv_name}" ]; then
      phase="$(kubectl get csv "${csv_name}" -n "${namespace}" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
      if [ "${phase}" = "Succeeded" ]; then
        echo "✓ CSV ${csv_name} is Succeeded"
        return 0
      fi
      if [ "${phase}" = "Failed" ]; then
        echo "✗ CSV ${csv_name} failed"
        kubectl get csv "${csv_name}" -n "${namespace}" -o yaml || true
        return 1
      fi
    fi
    sleep 5
  done

  echo "✗ Subscription ${subscription} did not install a Succeeded CSV"
  kubectl get subscription "${subscription}" -n "${namespace}" -o yaml || true
  kubectl get installplan,csv -n "${namespace}" || true
  return 1
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --ship)
      SHIP=true
      shift
      ;;
    --olm)
      OLM=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    -*)
      echo "Unknown flag: $1"
      usage
      exit 1
      ;;
    *)
      if [ -n "${TAG}" ]; then
        echo "Unexpected extra argument: $1"
        usage
        exit 1
      fi
      TAG="$1"
      shift
      ;;
  esac
done

if [ -z "${TAG}" ]; then
  TAG="v1.0.0-$(date +%Y%m%d%H%M%S)"
fi

OPERATOR_PUSH_IMAGE="${REGISTRY}/${OPERATOR_NAMESPACE}/${OPERATOR_IMAGE_NAME}:${TAG}"
PLUGIN_PUSH_IMAGE="${REGISTRY}/${PLUGIN_NAMESPACE}/${PLUGIN_IMAGE_NAME}:${TAG}"
BUNDLE_PUSH_IMAGE="${REGISTRY}/${OPERATOR_NAMESPACE}/${BUNDLE_IMAGE_NAME}:${TAG}"
CATALOG_PUSH_IMAGE="${REGISTRY}/${OPERATOR_NAMESPACE}/${CATALOG_IMAGE_NAME}:${TAG}"
OPERATOR_CLUSTER_IMAGE="${INTERNAL_REGISTRY}/${OPERATOR_NAMESPACE}/${OPERATOR_IMAGE_NAME}:${TAG}"
PLUGIN_CLUSTER_IMAGE="${INTERNAL_REGISTRY}/${PLUGIN_NAMESPACE}/${PLUGIN_IMAGE_NAME}:${TAG}"
BUNDLE_CLUSTER_IMAGE="${INTERNAL_REGISTRY}/${OPERATOR_NAMESPACE}/${BUNDLE_IMAGE_NAME}:${TAG}"
CATALOG_CLUSTER_IMAGE="${INTERNAL_REGISTRY}/${OPERATOR_NAMESPACE}/${CATALOG_IMAGE_NAME}:${TAG}"
DOCKERHUB_OPERATOR_IMAGE="docker.io/${DOCKERHUB_ORG}/${DOCKERHUB_OPERATOR_IMAGE_NAME}:${TAG}"
DOCKERHUB_PLUGIN_IMAGE="docker.io/${DOCKERHUB_ORG}/${DOCKERHUB_PLUGIN_IMAGE_NAME}:${TAG}"

echo "=========================================="
echo "Building Yamlwrangler Dashboard Operator"
echo "=========================================="
echo "Operator push image: ${OPERATOR_PUSH_IMAGE}"
echo "Operator cluster image: ${OPERATOR_CLUSTER_IMAGE}"
echo "Plugin push image:   ${PLUGIN_PUSH_IMAGE}"
echo "Plugin cluster image:${PLUGIN_CLUSTER_IMAGE}"
if [ "${OLM}" = true ]; then
  echo "Bundle push image:   ${BUNDLE_PUSH_IMAGE}"
  echo "Bundle cluster image:${BUNDLE_CLUSTER_IMAGE}"
  echo "Catalog push image:  ${CATALOG_PUSH_IMAGE}"
  echo "Catalog cluster image:${CATALOG_CLUSTER_IMAGE}"
fi
if [ "${SHIP}" = true ]; then
  echo "Docker Hub operator image: ${DOCKERHUB_OPERATOR_IMAGE}"
  echo "Docker Hub plugin image:   ${DOCKERHUB_PLUGIN_IMAGE}"
fi
if [ "${OLM}" = true ]; then
  echo "Install mode: OLM"
else
  echo "Install mode: raw manifests"
fi
echo ""

# Step 1: Build the Go binary
echo "Step 1: Building Go binary..."
make build
echo "✓ Binary built successfully"
echo ""

# Step 2: Create namespaces before pushing to the internal registry
echo "Step 2: Ensuring namespaces exist..."
kubectl get namespace "${OPERATOR_NAMESPACE}" 2>/dev/null || kubectl create namespace "${OPERATOR_NAMESPACE}"
kubectl get namespace "${PLUGIN_NAMESPACE}" 2>/dev/null || kubectl create namespace "${PLUGIN_NAMESPACE}"
echo "✓ Namespaces ready"
echo ""

# Step 3: Login to OpenShift registry
echo "Step 3: Logging in to OpenShift registry..."
oc registry login
echo "✓ Logged in successfully"
echo ""

# Step 4: Build the operator image
echo "Step 4: Building operator image..."
podman build -t "${OPERATOR_PUSH_IMAGE}" .
echo "✓ Operator image built successfully"
echo ""

# Step 5: Build the console plugin image
echo "Step 5: Building console plugin image..."
podman build --platform linux/amd64 -t "${PLUGIN_PUSH_IMAGE}" console-plugin
echo "✓ Console plugin image built successfully"
echo ""

# Step 6: Push images
echo "Step 6: Pushing images to registry..."
podman push "${OPERATOR_PUSH_IMAGE}"
podman push "${PLUGIN_PUSH_IMAGE}"
echo "✓ Images pushed successfully"
echo ""

if [ "${OLM}" = true ]; then
  echo "Step 7: Building and pushing OLM bundle/catalog images..."
  CSV_VERSION="${TAG#v}"
  CSV_NAME="${PACKAGE_NAME}.v${CSV_VERSION}"
  CATALOG_SOURCE_NAME="${PACKAGE_NAME}-catalog"
  TMP_OLM="$(mktemp -d)"
  trap 'rm -rf "${TMP_OLM}"' EXIT

  EXISTING_CSV=$(kubectl get csv -n "${OPERATOR_NAMESPACE}" -o name 2>/dev/null | grep "${PACKAGE_NAME}" | head -n 1 | cut -d'/' -f2 || echo "")

  sed \
    -e "s|name: app-dashboard-operator\.v.*|name: ${CSV_NAME}|" \
    -e "s|containerImage: .*|containerImage: ${OPERATOR_CLUSTER_IMAGE}|" \
    -e "s|image: fumbles/yamlwrangler-operator:.*|image: ${OPERATOR_CLUSTER_IMAGE}|" \
    -e "s|image: fumbles/yamlwrangler-dashboard:.*|image: ${PLUGIN_CLUSTER_IMAGE}|" \
    manifests/olm/clusterserviceversion.yaml.template |
    awk -v version="${CSV_VERSION}" '
      !replaced && /^  version:/ {
        print "  version: " version
        replaced = 1
        next
      }
      { print }
    ' > "${TMP_OLM}/${CSV_NAME}.clusterserviceversion.yaml"

  if [ -n "${EXISTING_CSV}" ] && [ "${EXISTING_CSV}" != "${CSV_NAME}" ]; then
    echo "Found existing CSV: ${EXISTING_CSV}, setting replaces for ${CSV_NAME}"
    awk -v existing="${EXISTING_CSV}" '/^  version:/ {print; print "  replaces: " existing; next} 1' \
      "${TMP_OLM}/${CSV_NAME}.clusterserviceversion.yaml" > "${TMP_OLM}/${CSV_NAME}.clusterserviceversion.yaml.tmp"
    mv "${TMP_OLM}/${CSV_NAME}.clusterserviceversion.yaml.tmp" "${TMP_OLM}/${CSV_NAME}.clusterserviceversion.yaml"
  fi

  mkdir -p "${TMP_OLM}/bundle/manifests" "${TMP_OLM}/bundle/metadata" "${TMP_OLM}/catalog"
  cp "${TMP_OLM}/${CSV_NAME}.clusterserviceversion.yaml" "${TMP_OLM}/bundle/manifests/"
  cp manifests/crds/*.yaml "${TMP_OLM}/bundle/manifests/"
  cat > "${TMP_OLM}/bundle/metadata/annotations.yaml" <<EOF
annotations:
  operators.operatorframework.io.bundle.mediatype.v1: registry+v1
  operators.operatorframework.io.bundle.manifests.v1: manifests/
  operators.operatorframework.io.bundle.metadata.v1: metadata/
  operators.operatorframework.io.bundle.package.v1: ${PACKAGE_NAME}
  operators.operatorframework.io.bundle.channels.v1: ${CHANNEL}
  operators.operatorframework.io.bundle.channel.default.v1: ${CHANNEL}
EOF
  cat > "${TMP_OLM}/bundle.Dockerfile" <<EOF
FROM scratch
LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=${PACKAGE_NAME}
LABEL operators.operatorframework.io.bundle.channels.v1=${CHANNEL}
LABEL operators.operatorframework.io.bundle.channel.default.v1=${CHANNEL}
COPY bundle/manifests /manifests
COPY bundle/metadata /metadata
EOF

  CHANNEL_REPLACES=""
  if [ -n "${EXISTING_CSV}" ] && [ "${EXISTING_CSV}" != "${CSV_NAME}" ]; then
    CHANNEL_REPLACES="    replaces: ${EXISTING_CSV}"
  fi

  cat > "${TMP_OLM}/catalog/catalog.yaml" <<EOF
schema: olm.package
name: ${PACKAGE_NAME}
defaultChannel: ${CHANNEL}
---
schema: olm.channel
package: ${PACKAGE_NAME}
name: ${CHANNEL}
entries:
  - name: ${CSV_NAME}
${CHANNEL_REPLACES}
---
schema: olm.bundle
name: ${CSV_NAME}
package: ${PACKAGE_NAME}
image: ${BUNDLE_CLUSTER_IMAGE}
properties:
  - type: olm.package
    value:
      packageName: ${PACKAGE_NAME}
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
relatedImages:
  - name: operator
    image: ${OPERATOR_CLUSTER_IMAGE}
  - name: dashboard
    image: ${PLUGIN_CLUSTER_IMAGE}
EOF
  cat > "${TMP_OLM}/catalog.Dockerfile" <<EOF
FROM quay.io/operator-framework/opm:latest
COPY catalog /configs
ENTRYPOINT ["/bin/opm"]
CMD ["serve", "/configs"]
EOF

  podman build --platform linux/amd64 -f "${TMP_OLM}/bundle.Dockerfile" -t "${BUNDLE_PUSH_IMAGE}" "${TMP_OLM}"
  podman push "${BUNDLE_PUSH_IMAGE}"
  podman build --platform linux/amd64 -f "${TMP_OLM}/catalog.Dockerfile" -t "${CATALOG_PUSH_IMAGE}" "${TMP_OLM}"
  podman push "${CATALOG_PUSH_IMAGE}"
  echo "✓ OLM bundle and catalog images pushed successfully"
  echo ""
fi

if [ "${SHIP}" = true ]; then
  echo "Step 7b: Shipping images to Docker Hub..."
  podman tag "${OPERATOR_PUSH_IMAGE}" "${DOCKERHUB_OPERATOR_IMAGE}"
  podman tag "${PLUGIN_PUSH_IMAGE}" "${DOCKERHUB_PLUGIN_IMAGE}"
  podman push "${DOCKERHUB_OPERATOR_IMAGE}"
  podman push "${DOCKERHUB_PLUGIN_IMAGE}"
  echo "✓ Docker Hub images pushed successfully"
  echo ""
fi

if [ "${OLM}" = true ]; then
  echo "Step 8: Installing operator through OLM CatalogSource/Subscription..."
  CATALOG_SOURCE_NAME="${PACKAGE_NAME}-catalog"
  TMP_CATALOGSOURCE="$(mktemp)"
  TMP_SUBSCRIPTION="$(mktemp)"
  trap 'rm -f "${TMP_CATALOGSOURCE}" "${TMP_SUBSCRIPTION}"; rm -rf "${TMP_OLM:-}"' EXIT

  kubectl apply -f manifests/olm/operatorgroup.yaml
  if ! kubectl get csv -n "${OPERATOR_NAMESPACE}" -o name 2>/dev/null | grep -q "${PACKAGE_NAME}"; then
    echo "Cleaning raw operator workload/RBAC before OLM takes ownership..."
    kubectl delete deployment "${OPERATOR_IMAGE_NAME}" -n "${OPERATOR_NAMESPACE}" --ignore-not-found
    kubectl delete serviceaccount "${OPERATOR_IMAGE_NAME}" -n "${OPERATOR_NAMESPACE}" --ignore-not-found
    kubectl delete clusterrole "${OPERATOR_IMAGE_NAME}" --ignore-not-found
    kubectl delete clusterrolebinding "${OPERATOR_IMAGE_NAME}" --ignore-not-found
  fi

  sed \
    -e "s|name: app-dashboard-operator-catalog|name: ${CATALOG_SOURCE_NAME}|" \
    -e "s|namespace: app-dashboard-operator|namespace: ${OPERATOR_NAMESPACE}|" \
    -e "s|image: .*|image: ${CATALOG_CLUSTER_IMAGE}|" \
    -e "s|displayName: .*|displayName: ${PACKAGE_NAME} catalog|" \
    manifests/olm/catalogsource.yaml.template > "${TMP_CATALOGSOURCE}"
  sed \
    -e "s|namespace: app-dashboard-operator|namespace: ${OPERATOR_NAMESPACE}|" \
    -e "s|name: app-dashboard-operator$|name: ${PACKAGE_NAME}|" \
    -e "s|channel: .*|channel: ${CHANNEL}|" \
    -e "s|source: .*|source: ${CATALOG_SOURCE_NAME}|" \
    -e "s|sourceNamespace: .*|sourceNamespace: ${OPERATOR_NAMESPACE}|" \
    manifests/olm/subscription.yaml.template > "${TMP_SUBSCRIPTION}"

  kubectl apply -f "${TMP_CATALOGSOURCE}"
  wait_for_catalogsource "${CATALOG_SOURCE_NAME}" "${OPERATOR_NAMESPACE}"
  kubectl apply -f "${TMP_SUBSCRIPTION}"
  wait_for_subscription_csv "${PACKAGE_NAME}" "${OPERATOR_NAMESPACE}"
  echo "✓ OLM operator install complete"
else
  # Step 8: Install CRDs and deploy the operator
  echo "Step 8: Installing CRDs and deploying operator..."
  kubectl apply -f manifests/crds/

  # Wait for CRDs to be established before applying CRs
  echo "Waiting for CRDs to be established..."
  kubectl wait --for condition=established --timeout=60s \
    crd/appdashboards.dashboard.yamlwrangler.com \
    crd/dashboardappgroups.dashboard.yamlwrangler.com
  echo "✓ CRDs are established"

  kubectl apply -f manifests/deploy/
  kubectl set image "deployment/app-dashboard-operator" "manager=${OPERATOR_CLUSTER_IMAGE}" -n "${OPERATOR_NAMESPACE}"
  echo "✓ Operator manifests applied"
fi

echo ""
echo "Step 9: Waiting for operator to be ready..."
kubectl rollout status "deployment/app-dashboard-operator" -n "${OPERATOR_NAMESPACE}" --timeout=120s
echo "✓ Operator is ready"
echo ""

# Step 10: Apply the AppDashboard CR with the freshly built plugin image
echo "Step 10: Applying AppDashboard CR..."
TMP_DASHBOARD="$(mktemp)"
sed "s|image: .*yamlwrangler-dashboard:.*|image: ${PLUGIN_CLUSTER_IMAGE}|" manifests/samples/appdashboard.yaml > "${TMP_DASHBOARD}"
kubectl apply -f "${TMP_DASHBOARD}"
rm -f "${TMP_DASHBOARD}"
echo "✓ AppDashboard ${APP_DASHBOARD_NAME} applied"
echo ""

# Step 11: Wait for plugin deployment if the operator has created it
echo "Step 11: Waiting for console plugin deployment..."
kubectl rollout status "deployment/app-dashboard" -n "${PLUGIN_NAMESPACE}" --timeout=180s || {
  echo "⚠ Console plugin deployment was not ready yet. Check operator logs and AppDashboard status."
}
echo ""

echo "=========================================="
echo "Deployment Complete!"
echo "=========================================="
echo ""
echo "To view operator logs:"
echo "  kubectl logs -f deployment/app-dashboard-operator -n ${OPERATOR_NAMESPACE}"
echo ""
echo "To check operator status:"
echo "  kubectl get pods -n ${OPERATOR_NAMESPACE}"
if [ "${OLM}" = true ]; then
  echo "  oc get operator -n ${OPERATOR_NAMESPACE}"
  echo "  oc get csv -n ${OPERATOR_NAMESPACE}"
fi
echo ""
echo "To check dashboard status:"
echo "  kubectl get appdashboard ${APP_DASHBOARD_NAME} -o yaml"
echo "  kubectl get pods -n ${PLUGIN_NAMESPACE}"
echo ""
echo "To enable discovery in an app namespace:"
echo "  oc label namespace <namespace> dashboard.yamlwrangler.com/enabled=true"
echo ""

# Made with Bob
