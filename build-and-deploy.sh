#!/bin/bash

set -e

# Configuration
REGISTRY="default-route-openshift-image-registry.apps.sno.yamlwrangler.com"
NAMESPACE="app-dashboard-operator"
IMAGE_NAME="app-dashboard-operator"

# Use provided tag or generate timestamp-based tag
if [ -n "$1" ]; then
  TAG="$1"
else
  TAG="v1.0.0-$(date +%Y%m%d%H%M%S)"
fi

FULL_IMAGE="${REGISTRY}/${NAMESPACE}/${IMAGE_NAME}:${TAG}"

echo "=========================================="
echo "Building App Dashboard Operator"
echo "=========================================="
echo "Image: ${FULL_IMAGE}"
echo ""

# Step 1: Build the Go binary
echo "Step 1: Building Go binary..."
make build
echo "✓ Binary built successfully"
echo ""

# Step 2: Build the container image
echo "Step 2: Building container image..."
podman build -t ${FULL_IMAGE} .
echo "✓ Image built successfully"
echo ""

# Step 3: Login to OpenShift registry
echo "Step 3: Logging in to OpenShift registry..."
oc registry login
echo "✓ Logged in successfully"
echo ""

# Step 4: Push the image
echo "Step 4: Pushing image to registry..."
podman push ${FULL_IMAGE}
echo "✓ Image pushed successfully"
echo ""

# Step 5: Create namespace if it doesn't exist
echo "Step 5: Ensuring namespace exists..."
kubectl get namespace ${NAMESPACE} 2>/dev/null || kubectl create namespace ${NAMESPACE}
echo "✓ Namespace ready"
echo ""

# Step 6: Deploy the operator
echo "Step 6: Deploying operator..."
kubectl apply -f manifests/deploy/
echo "✓ Operator deployed"
echo ""

# Step 7: Wait for deployment to be ready
echo "Step 7: Waiting for operator to be ready..."
kubectl rollout status deployment/app-dashboard-operator -n ${NAMESPACE} --timeout=120s
echo "✓ Operator is ready"
echo ""

# Step 8: Show operator logs
echo "=========================================="
echo "Deployment Complete!"
echo "=========================================="
echo ""
echo "To view operator logs:"
echo "  kubectl logs -f deployment/app-dashboard-operator -n ${NAMESPACE}"
echo ""
echo "To check operator status:"
echo "  kubectl get pods -n ${NAMESPACE}"
echo ""
echo "To create an AppGroup:"
echo "  kubectl apply -f manifests/examples/plane-appgroup.yaml"
echo ""

# Made with Bob
