#!/bin/bash
# Test deploy: builds and deploys the operator into an isolated namespace,
# safe to run alongside the OLM-managed or production install.
#
# Usage:
#   ./build-and-deploy-test.sh [tag]
#
# Defaults:
#   TAG          v1.0.0-<timestamp>
#   OPERATOR_NS  dashboard-op-test
#   PLUGIN_NS    dashboard-test
#   CR_NAME      test-dashboard

set -euo pipefail

TAG="${1:-v1.0.0-$(date +%Y%m%d%H%M%S)}"
OPERATOR_NS="${OPERATOR_NS:-dashboard-op-test}"
PLUGIN_NS="${PLUGIN_NS:-dashboard-test}"
CR_NAME="${CR_NAME:-test-dashboard}"
PLUGIN_NAME="${PLUGIN_NAME:-app-dashboard-test}"

REGISTRY="${REGISTRY:-image-registry.openshift-image-registry.svc:5000}"
OPERATOR_IMAGE="${REGISTRY}/${OPERATOR_NS}/app-dashboard-operator:${TAG}"
PLUGIN_IMAGE="${REGISTRY}/${PLUGIN_NS}/app-dashboard-console-plugin:${TAG}"
PUSH_REGISTRY="${PUSH_REGISTRY:-default-route-openshift-image-registry.apps.sno.yamlwrangler.com}"
PUSH_OPERATOR_IMAGE="${PUSH_REGISTRY}/${OPERATOR_NS}/app-dashboard-operator:${TAG}"
PUSH_PLUGIN_IMAGE="${PUSH_REGISTRY}/${PLUGIN_NS}/app-dashboard-console-plugin:${TAG}"

echo "============================================"
echo "Test deploy — isolated namespaces"
echo "============================================"
echo "  Tag:          ${TAG}"
echo "  Operator NS:  ${OPERATOR_NS}"
echo "  Plugin NS:    ${PLUGIN_NS}"
echo "  Plugin name:  ${PLUGIN_NAME}"
echo "  CR name:      ${CR_NAME}"
echo "  Operator img: ${OPERATOR_IMAGE}"
echo "  Plugin img:   ${PLUGIN_IMAGE}"
echo ""

# Step 1: Build Go binary
echo "Step 1: Building Go binary..."
make build
echo "✓ Binary built"
echo ""

# Step 2: Ensure namespaces exist (registry needs them before push)
echo "Step 2: Ensuring namespaces exist..."
kubectl get namespace "${OPERATOR_NS}" 2>/dev/null || kubectl create namespace "${OPERATOR_NS}"
kubectl get namespace "${PLUGIN_NS}" 2>/dev/null || kubectl create namespace "${PLUGIN_NS}"
echo "✓ Namespaces ready"
echo ""

# Step 3: Login + build + push images
echo "Step 3: Logging in to OpenShift registry..."
oc registry login
echo "✓ Logged in"
echo ""

echo "Step 4: Building operator image..."
podman build -t "${PUSH_OPERATOR_IMAGE}" .
echo "✓ Operator image built"
echo ""

echo "Step 5: Building console plugin image..."
podman build --platform linux/amd64 -t "${PUSH_PLUGIN_IMAGE}" console-plugin
echo "✓ Plugin image built"
echo ""

echo "Step 6: Pushing images..."
podman push "${PUSH_OPERATOR_IMAGE}"
podman push "${PUSH_PLUGIN_IMAGE}"
echo "✓ Images pushed"
echo ""

# Step 7: Install CRDs (shared, safe to re-apply)
echo "Step 7: Installing CRDs..."
kubectl apply -f manifests/crds/
kubectl wait --for condition=established --timeout=60s \
  crd/appdashboards.dashboard.yamlwrangler.com \
  crd/dashboardappgroups.dashboard.yamlwrangler.com \
  crd/dashboardnamespaceconfigs.dashboard.yamlwrangler.com \
  crd/dashboardlinks.dashboard.yamlwrangler.com
echo "✓ CRDs established"
echo ""

# Step 8: Deploy operator into test namespace (sed-patch manifests, don't edit source)
echo "Step 8: Deploying operator into ${OPERATOR_NS}..."
sed "s/namespace: app-dashboard-operator/namespace: ${OPERATOR_NS}/g" \
  manifests/deploy/namespace.yaml | kubectl apply -f -
sed "s/namespace: app-dashboard-operator/namespace: ${OPERATOR_NS}/g" \
  manifests/deploy/serviceaccount.yaml | kubectl apply -f -
sed "s/name: app-dashboard-operator/name: app-dashboard-operator-${OPERATOR_NS}/g" \
  manifests/deploy/clusterrole.yaml | kubectl apply -f -
sed \
  -e "s/name: app-dashboard-operator$/name: app-dashboard-operator-${OPERATOR_NS}/g" \
  -e "s/namespace: app-dashboard-operator/namespace: ${OPERATOR_NS}/g" \
  manifests/deploy/clusterrolebinding.yaml | kubectl apply -f -
sed \
  -e "s/namespace: app-dashboard-operator/namespace: ${OPERATOR_NS}/g" \
  -e "s|image: .*yamlwrangler-operator:.*|image: ${OPERATOR_IMAGE}|" \
  manifests/deploy/deployment.yaml | kubectl apply -f -
echo "✓ Operator manifests applied"
echo ""

# Step 9: Wait for operator
echo "Step 9: Waiting for operator rollout..."
kubectl rollout status deployment/app-dashboard-operator -n "${OPERATOR_NS}" --timeout=120s
echo "✓ Operator ready"
echo ""

# Step 10: Apply AppDashboard CR
echo "Step 10: Applying AppDashboard CR ${CR_NAME}..."
sed \
  -e "s/name: yamlwrangler/name: ${CR_NAME}/" \
  -e "s/pluginName: app-dashboard/pluginName: ${PLUGIN_NAME}/" \
  -e "s|image: .*yamlwrangler-dashboard:.*|image: ${PLUGIN_IMAGE}|" \
  -e "s/namespace: app-dashboard/namespace: ${PLUGIN_NS}/" \
  manifests/samples/appdashboard.yaml | kubectl apply -f -
echo "✓ AppDashboard CR applied"
echo ""

# Step 11: Wait for plugin
echo "Step 11: Waiting for console plugin deployment..."
kubectl rollout status deployment/app-dashboard -n "${PLUGIN_NS}" --timeout=180s || {
  echo "⚠  Plugin deployment not ready yet — check operator logs and AppDashboard status."
}
echo ""

echo "============================================"
echo "Test deploy complete!"
echo "============================================"
echo ""
echo "Operator logs:  kubectl logs -f deployment/app-dashboard-operator -n ${OPERATOR_NS}"
echo "Plugin pods:    kubectl get pods -n ${PLUGIN_NS}"
echo "CR status:      kubectl get appdashboard ${CR_NAME} -o yaml"
echo ""
echo "To tear down:"
echo "  kubectl delete appdashboard ${CR_NAME} --ignore-not-found"
echo "  kubectl delete namespace ${PLUGIN_NS} --ignore-not-found"
echo "  kubectl delete namespace ${OPERATOR_NS} --ignore-not-found"
echo "  kubectl delete clusterrole app-dashboard-operator-${OPERATOR_NS} --ignore-not-found"
echo "  kubectl delete clusterrolebinding app-dashboard-operator-${OPERATOR_NS} --ignore-not-found"
