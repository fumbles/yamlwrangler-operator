#!/bin/bash

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Change to the script directory
cd "$SCRIPT_DIR" || exit 1

# Extract version from package.json
PACKAGE_VERSION=$(node -p "require('./package.json').version")

# Use provided version or default to package.json version with timestamp
if [ -n "$1" ]; then
  VERSION="$1"
else
  VERSION="${PACKAGE_VERSION}-$(date +%Y%m%d%H%M%S)"
fi

TAG="amd64-${VERSION}"

printf "Building version: %s\n" "$VERSION"
printf "Full tag: %s\n" "$TAG"

# Set variables
printf "\nSetting variables...\n"
REGISTRY=$(oc get route default-route -n openshift-image-registry -o jsonpath='{.spec.host}')
TOKEN=$(oc whoami -t)

# Rebuild plugin assets
printf "\nRebuilding plugin assets...\n"
yarn build

# Build amd64 image
printf "\nBuilding amd64 image...\n"
podman build --platform linux/amd64 \
  -t app-dashboard-console-plugin:$TAG .

# Login to OpenShift registry
printf "\nLogging in to OpenShift registry...\n"
podman login -u "$(oc whoami)" -p "$TOKEN" \
  --tls-verify=false "$REGISTRY"

# Tag image
printf "\nTagging image...\n"
podman tag app-dashboard-console-plugin:$TAG \
  "$REGISTRY/app-dashboard/app-dashboard-console-plugin:$TAG"

# Push image
printf "\nPushing image...\n"
podman push --tls-verify=false \
  "$REGISTRY/app-dashboard/app-dashboard-console-plugin:$TAG"

# Upgrade Helm deployment
printf "\nUpgrading Helm deployment...\n"
helm upgrade app-dashboard \
  ./charts/openshift-console-plugin \
  -n app-dashboard \
  --set plugin.name=app-dashboard \
  --set plugin.description="Yamlwrangler App Dashboard" \
  --set plugin.image=image-registry.openshift-image-registry.svc:5000/app-dashboard/app-dashboard-console-plugin:$TAG

printf "\n✅ Deployment complete!\n"
printf "Version: %s\n" "$VERSION"
printf "Tag: %s\n" "$TAG"
