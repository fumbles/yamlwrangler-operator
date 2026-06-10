#!/bin/bash
# Helper script to label routes for the App Dashboard

# Usage: ./label-routes.sh <namespace> <route-name> <display-name> <category> <description>

if [ $# -lt 4 ]; then
  echo "Usage: $0 <namespace> <route-name> <display-name> <category> [description]"
  echo ""
  echo "Categories:"
  echo "  - Infrastructure"
  echo "  - Services"
  echo "  - Media"
  echo "  - AI / Experimental"
  echo ""
  echo "Example:"
  echo "  $0 lab linkding Linkding Services 'Bookmark manager'"
  exit 1
fi

NAMESPACE=$1
ROUTE=$2
DISPLAY_NAME=$3
CATEGORY=$4
DESCRIPTION=${5:-""}

echo "Labeling route: $ROUTE in namespace: $NAMESPACE"
echo "  Display Name: $DISPLAY_NAME"
echo "  Category: $CATEGORY"
echo "  Description: $DESCRIPTION"
echo ""

# Add the label
oc -n "$NAMESPACE" label route "$ROUTE" dashboard.yamlwrangler.com/enabled=true --overwrite

# Add annotations
if [ -n "$DESCRIPTION" ]; then
  oc -n "$NAMESPACE" annotate route "$ROUTE" \
    dashboard.yamlwrangler.com/display-name="$DISPLAY_NAME" \
    dashboard.yamlwrangler.com/category="$CATEGORY" \
    dashboard.yamlwrangler.com/description="$DESCRIPTION" \
    --overwrite
else
  oc -n "$NAMESPACE" annotate route "$ROUTE" \
    dashboard.yamlwrangler.com/display-name="$DISPLAY_NAME" \
    dashboard.yamlwrangler.com/category="$CATEGORY" \
    --overwrite
fi

echo ""
echo "✓ Route labeled successfully!"
echo "The dashboard will automatically update."

# Made with Bob
