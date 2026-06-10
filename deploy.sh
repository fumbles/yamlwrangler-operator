#!/bin/bash

# App Dashboard Console Plugin Deployment Script
# Deploys the dashboard plugin from Docker Hub using Helm

set -e

VERSION="${1:-v1.0.0}"
IMAGE="fumbles/yamlwrangler-dashboard:${VERSION}"
NAMESPACE="app-dashboard-plugin"

echo "=========================================="
echo "App Dashboard Console Plugin Deployment"
echo "=========================================="
echo "Version: $VERSION"
echo "Image: $IMAGE"
echo ""

# Pre-flight checks
echo "📋 Pre-flight Checks"
echo "-------------------"

# Check if oc/kubectl is available
if command -v oc &> /dev/null; then
    CLI="oc"
    echo "✓ OpenShift CLI (oc) found"
elif command -v kubectl &> /dev/null; then
    CLI="kubectl"
    echo "✓ Kubernetes CLI (kubectl) found"
else
    echo "✗ Error: Neither 'oc' nor 'kubectl' found"
    echo "  Please install OpenShift or Kubernetes CLI"
    exit 1
fi

# Check if helm is available
if ! command -v helm &> /dev/null; then
    echo "✗ Error: Helm not found"
    echo "  Please install Helm 3.x: https://helm.sh/docs/intro/install/"
    exit 1
fi
echo "✓ Helm found ($(helm version --short))"

# Check cluster connection
if ! $CLI cluster-info &> /dev/null; then
    echo "✗ Error: Not connected to a cluster"
    echo "  Run: oc login <cluster-url>"
    exit 1
fi
echo "✓ Connected to cluster"

# Check if this is OpenShift
if ! $CLI api-resources | grep -q "console.openshift.io"; then
    echo "⚠ Warning: This doesn't appear to be an OpenShift cluster"
    echo "  This plugin requires OpenShift Console"
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi
echo "✓ OpenShift cluster detected"

echo ""
echo "📦 Deployment Steps"
echo "-------------------"

# Check if already installed
if helm list -n $NAMESPACE 2>/dev/null | grep -q "app-dashboard"; then
    echo "ℹ️  Existing installation found"
    read -p "Upgrade existing installation? (Y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Nn]$ ]]; then
        echo "Deployment cancelled"
        exit 0
    fi
    ACTION="upgrade"
else
    ACTION="install"
fi

# Deploy with Helm
echo "1. Deploying plugin with Helm..."
if [ "$ACTION" = "upgrade" ]; then
    helm upgrade app-dashboard charts/openshift-console-plugin \
        --namespace $NAMESPACE \
        --set plugin.image=$IMAGE \
        --reuse-values=false \
        --wait
else
    helm install app-dashboard charts/openshift-console-plugin \
        --create-namespace \
        --namespace $NAMESPACE \
        --set plugin.image=$IMAGE \
        --wait
fi
echo "✓ Helm deployment complete"

# Wait for pods
echo -n "2. Waiting for pods to be ready... "
$CLI wait --for=condition=ready --timeout=120s \
    pod -l app=app-dashboard -n $NAMESPACE > /dev/null 2>&1 && echo "✓" || echo "⚠ (timeout, check manually)"

echo ""
echo "=========================================="
echo "✅ Deployment Complete!"
echo "=========================================="
echo ""

# Show status
echo "📊 Plugin Status:"
$CLI get pods -n $NAMESPACE
echo ""

# Check ConsolePlugin
echo "📝 ConsolePlugin Status:"
if $CLI get consoleplugin app-dashboard &> /dev/null; then
    echo "✓ ConsolePlugin 'app-dashboard' exists"
else
    echo "⚠ ConsolePlugin 'app-dashboard' not found"
fi
echo ""

# Show logs command
echo "📝 View Logs:"
echo "  $CLI logs -f deployment/app-dashboard -n $NAMESPACE"
echo ""

# Next steps
echo "🚀 Next Steps:"
echo "-------------------"
echo "1. Add Application Menu Link (optional):"
echo "   Update console-link.yaml with your console URL, then:"
echo "   $CLI apply -f console-link.yaml"
echo ""
echo "2. Get your console URL:"
echo "   $CLI get route console -n openshift-console -o jsonpath='{.spec.host}'"
echo ""
echo "3. Access the dashboard:"
echo "   Open OpenShift Console → Application Menu (9 dots) → App Dashboard"
echo "   Or navigate to: https://<console-url>/app-dashboard"
echo ""
echo "4. Hard refresh your browser:"
echo "   Press Ctrl+Shift+R (Windows/Linux) or Cmd+Shift+R (Mac)"
echo ""
echo "5. If plugin doesn't load, enable it manually:"
echo "   $CLI patch console.operator.openshift.io cluster \\"
echo "     --type='json' \\"
echo "     -p='[{\"op\": \"add\", \"path\": \"/spec/plugins/-\", \"value\": \"app-dashboard\"}]'"
echo ""
echo "6. Deploy the Operator for automatic discovery:"
echo "   https://github.com/fumbles/yamlwrangler-operator"
echo ""

# Made with Bob