#!/bin/bash

# Script to add a custom link to the App Dashboard
# Usage: ./add-custom-link.sh <namespace> <name> "<display-name>" "<category>" "<description>" "<url>"

set -e

if [ "$#" -ne 6 ]; then
    echo "Usage: $0 <namespace> <name> \"<display-name>\" \"<category>\" \"<description>\" \"<url>\""
    echo ""
    echo "Example:"
    echo "  $0 twingate twingate-link \"Twingate\" \"Infrastructure\" \"Zero Trust Network Access\" \"https://twingate.example.com\""
    echo ""
    echo "Categories:"
    echo "  - Infrastructure"
    echo "  - Services"
    echo "  - Media"
    echo "  - AI / Experimental"
    echo "  - Development"
    echo "  - Monitoring"
    echo "  - Games"
    exit 1
fi

NAMESPACE="$1"
NAME="$2"
DISPLAY_NAME="$3"
CATEGORY="$4"
DESCRIPTION="$5"
URL="$6"

echo "Creating custom link ConfigMap: $NAME in namespace: $NAMESPACE"
echo "  Display Name: $DISPLAY_NAME"
echo "  Category: $CATEGORY"
echo "  Description: $DESCRIPTION"
echo "  URL: $URL"
echo ""

# Create the ConfigMap
cat <<EOF | oc apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: $NAME
  namespace: $NAMESPACE
  labels:
    dashboard.yamlwrangler.com/type: custom-link
data:
  displayName: "$DISPLAY_NAME"
  category: "$CATEGORY"
  description: "$DESCRIPTION"
  url: "$URL"
EOF

if [ $? -eq 0 ]; then
    echo ""
    echo "✓ Custom link created successfully!"
    echo "The dashboard will automatically update."
    echo ""
    echo "To remove this link later:"
    echo "  oc delete configmap $NAME -n $NAMESPACE"
else
    echo ""
    echo "✗ Failed to create custom link"
    exit 1
fi

# Made with Bob
