#!/bin/bash

set -e

# Configuration
REGISTRY="${REGISTRY:-default-route-openshift-image-registry.apps.sno.yamlwrangler.com}"
INTERNAL_REGISTRY="${INTERNAL_REGISTRY:-image-registry.openshift-image-registry.svc:5000}"
OPERATOR_NAMESPACE="${OPERATOR_NAMESPACE:-app-dashboard-operator}"
PLUGIN_NAMESPACE="${PLUGIN_NAMESPACE:-app-dashboard}"
OPERATOR_IMAGE_NAME="${OPERATOR_IMAGE_NAME:-app-dashboard-operator}"
PLUGIN_IMAGE_NAME="${PLUGIN_IMAGE_NAME:-app-dashboard-console-plugin}"
APP_DASHBOARD_NAME="${APP_DASHBOARD_NAME:-yamlwrangler}"
DOCKERHUB_ORG="${DOCKERHUB_ORG:-fumbles}"
DOCKERHUB_OPERATOR_IMAGE_NAME="${DOCKERHUB_OPERATOR_IMAGE_NAME:-yamlwrangler-operator}"
DOCKERHUB_PLUGIN_IMAGE_NAME="${DOCKERHUB_PLUGIN_IMAGE_NAME:-yamlwrangler-dashboard}"
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
EOF
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
OPERATOR_CLUSTER_IMAGE="${INTERNAL_REGISTRY}/${OPERATOR_NAMESPACE}/${OPERATOR_IMAGE_NAME}:${TAG}"
PLUGIN_CLUSTER_IMAGE="${INTERNAL_REGISTRY}/${PLUGIN_NAMESPACE}/${PLUGIN_IMAGE_NAME}:${TAG}"
DOCKERHUB_OPERATOR_IMAGE="docker.io/${DOCKERHUB_ORG}/${DOCKERHUB_OPERATOR_IMAGE_NAME}:${TAG}"
DOCKERHUB_PLUGIN_IMAGE="docker.io/${DOCKERHUB_ORG}/${DOCKERHUB_PLUGIN_IMAGE_NAME}:${TAG}"

echo "=========================================="
echo "Building Yamlwrangler Dashboard Operator"
echo "=========================================="
echo "Operator push image: ${OPERATOR_PUSH_IMAGE}"
echo "Operator cluster image: ${OPERATOR_CLUSTER_IMAGE}"
echo "Plugin push image:   ${PLUGIN_PUSH_IMAGE}"
echo "Plugin cluster image:${PLUGIN_CLUSTER_IMAGE}"
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

if [ "${SHIP}" = true ]; then
  echo "Step 7: Shipping images to Docker Hub..."
  podman tag "${OPERATOR_PUSH_IMAGE}" "${DOCKERHUB_OPERATOR_IMAGE}"
  podman tag "${PLUGIN_PUSH_IMAGE}" "${DOCKERHUB_PLUGIN_IMAGE}"
  podman push "${DOCKERHUB_OPERATOR_IMAGE}"
  podman push "${DOCKERHUB_PLUGIN_IMAGE}"
  echo "✓ Docker Hub images pushed successfully"
  echo ""
fi

# Step 8: Install CRDs and deploy the operator
echo "Step 8: Installing CRDs and deploying operator..."
kubectl apply -f manifests/crds/
if [ "${OLM}" = true ]; then
  kubectl apply -f manifests/olm/operatorgroup.yaml
  kubectl apply -f manifests/olm/serviceaccount.yaml
  kubectl apply -f manifests/deploy/clusterrole.yaml
  kubectl apply -f manifests/deploy/clusterrolebinding.yaml
  TMP_CSV="$(mktemp)"
  sed \
    -e "s|containerImage: .*|containerImage: ${OPERATOR_CLUSTER_IMAGE}|" \
    -e "s|image: fumbles/yamlwrangler-operator:.*|image: ${OPERATOR_CLUSTER_IMAGE}|" \
    manifests/olm/app-dashboard-operator.clusterserviceversion.yaml > "${TMP_CSV}"
  kubectl apply -f "${TMP_CSV}"
  rm -f "${TMP_CSV}"
  echo "✓ OperatorGroup and CSV applied"
else
  kubectl apply -f manifests/deploy/
  kubectl set image "deployment/app-dashboard-operator" "manager=${OPERATOR_CLUSTER_IMAGE}" -n "${OPERATOR_NAMESPACE}"
  echo "✓ Operator manifests applied"
fi
echo ""

# Step 9: Wait for operator deployment to be ready
echo "Step 9: Waiting for operator to be ready..."
if [ "${OLM}" = true ]; then
  CSV_READY=false
  for _ in {1..60}; do
    CSV_PHASE="$(kubectl get csv app-dashboard-operator.v0.1.0 -n "${OPERATOR_NAMESPACE}" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
    if [ "${CSV_PHASE}" = "Succeeded" ]; then
      echo "✓ CSV is Succeeded"
      CSV_READY=true
      break
    fi
    if [ "${CSV_PHASE}" = "Failed" ]; then
      echo "✗ CSV failed"
      kubectl get csv app-dashboard-operator.v0.1.0 -n "${OPERATOR_NAMESPACE}" -o yaml
      exit 1
    fi
    sleep 5
  done
  if [ "${CSV_READY}" != true ]; then
    echo "✗ CSV did not reach Succeeded within timeout"
    kubectl get csv app-dashboard-operator.v0.1.0 -n "${OPERATOR_NAMESPACE}" -o yaml || true
    exit 1
  fi
fi
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
