#!/bin/bash

# Script to prune old image stream tags, keeping only the latest N versions
# Usage: ./prune-images.sh [namespace] [imagestream-name] [keep-count]
# Example: ./prune-images.sh openshift app-dashboard-console-plugin 2

set -e

# Default values
KEEP_COUNT=2
DRY_RUN=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Function to show usage
show_usage() {
    cat << EOF
Usage: $0 [OPTIONS] <namespace> <imagestream-name> [keep-count]

Prune old image stream tags, keeping only the latest N versions.

Arguments:
  namespace         The namespace containing the image stream
  imagestream-name  The name of the image stream to prune
  keep-count        Number of recent tags to keep (default: 2)

Options:
  --dry-run         Show what would be deleted without actually deleting
  -h, --help        Show this help message

Examples:
  # Keep only the 2 most recent tags (default)
  $0 openshift app-dashboard-console-plugin

  # Keep only the 3 most recent tags
  $0 openshift app-dashboard-console-plugin 3

  # Dry run to see what would be deleted
  $0 --dry-run openshift app-dashboard-console-plugin 2

EOF
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        *)
            if [ -z "$NAMESPACE" ]; then
                NAMESPACE="$1"
            elif [ -z "$IMAGESTREAM" ]; then
                IMAGESTREAM="$1"
            elif [ -z "$KEEP_COUNT_ARG" ]; then
                KEEP_COUNT_ARG="$1"
                if ! [[ "$KEEP_COUNT_ARG" =~ ^[0-9]+$ ]]; then
                    print_error "Keep count must be a positive number"
                    exit 1
                fi
                KEEP_COUNT="$KEEP_COUNT_ARG"
            else
                print_error "Too many arguments"
                show_usage
                exit 1
            fi
            shift
            ;;
    esac
done

# Validate required arguments
if [ -z "$NAMESPACE" ] || [ -z "$IMAGESTREAM" ]; then
    print_error "Missing required arguments"
    show_usage
    exit 1
fi

# Validate keep count
if [ "$KEEP_COUNT" -lt 1 ]; then
    print_error "Keep count must be at least 1"
    exit 1
fi

print_info "Namespace: $NAMESPACE"
print_info "Image Stream: $IMAGESTREAM"
print_info "Keeping: $KEEP_COUNT most recent tag(s)"
if [ "$DRY_RUN" = true ]; then
    print_warning "DRY RUN MODE - No changes will be made"
fi
echo ""

# Check if image stream exists
if ! oc get imagestream "$IMAGESTREAM" -n "$NAMESPACE" &>/dev/null; then
    print_error "Image stream '$IMAGESTREAM' not found in namespace '$NAMESPACE'"
    exit 1
fi

# Get all tags sorted by creation time (newest first)
print_info "Fetching image stream tags..."
TAGS=$(oc get imagestreamtag -n "$NAMESPACE" -o json | \
    jq -r --arg is "$IMAGESTREAM" '.items[] | 
    select(.metadata.name | startswith($is + ":")) | 
    {name: .metadata.name, created: .metadata.creationTimestamp} | 
    @json' | \
    jq -s 'sort_by(.created) | reverse | .[].name' -r)

if [ -z "$TAGS" ]; then
    print_warning "No tags found for image stream '$IMAGESTREAM'"
    exit 0
fi

# Convert to array
TAG_ARRAY=()
while IFS= read -r tag; do
    TAG_ARRAY+=("$tag")
done <<< "$TAGS"

TOTAL_TAGS=${#TAG_ARRAY[@]}
print_success "Found $TOTAL_TAGS tag(s)"
echo ""

# Display all tags
print_info "Current tags (newest to oldest):"
for i in "${!TAG_ARRAY[@]}"; do
    TAG="${TAG_ARRAY[$i]}"
    TAG_NAME="${TAG#*:}"
    if [ $i -lt $KEEP_COUNT ]; then
        echo -e "  ${GREEN}✓${NC} $TAG_NAME (keeping)"
    else
        echo -e "  ${RED}✗${NC} $TAG_NAME (will be deleted)"
    fi
done
echo ""

# Calculate tags to delete
if [ $TOTAL_TAGS -le $KEEP_COUNT ]; then
    print_success "No tags to delete. Current count ($TOTAL_TAGS) <= keep count ($KEEP_COUNT)"
    exit 0
fi

TAGS_TO_DELETE=$((TOTAL_TAGS - KEEP_COUNT))
print_warning "Will delete $TAGS_TO_DELETE tag(s)"
echo ""

# Delete old tags
if [ "$DRY_RUN" = true ]; then
    print_info "DRY RUN - Would delete the following tags:"
    for ((i=$KEEP_COUNT; i<$TOTAL_TAGS; i++)); do
        TAG="${TAG_ARRAY[$i]}"
        echo "  - $TAG"
    done
else
    # Ask for confirmation
    read -p "$(echo -e ${YELLOW}Are you sure you want to delete $TAGS_TO_DELETE tag\(s\)? \(yes/no\): ${NC})" -r
    echo
    if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
        print_info "Aborted by user"
        exit 0
    fi

    print_info "Deleting old tags..."
    DELETED_COUNT=0
    FAILED_COUNT=0
    
    for ((i=$KEEP_COUNT; i<$TOTAL_TAGS; i++)); do
        TAG="${TAG_ARRAY[$i]}"
        TAG_NAME="${TAG#*:}"
        
        if oc delete imagestreamtag "$TAG" -n "$NAMESPACE" 2>/dev/null; then
            print_success "Deleted: $TAG_NAME"
            ((DELETED_COUNT++))
        else
            print_error "Failed to delete: $TAG_NAME"
            ((FAILED_COUNT++))
        fi
    done
    
    echo ""
    print_success "Deleted $DELETED_COUNT tag(s)"
    if [ $FAILED_COUNT -gt 0 ]; then
        print_warning "Failed to delete $FAILED_COUNT tag(s)"
    fi
fi

# Show remaining tags
echo ""
print_info "Remaining tags:"
REMAINING_TAGS=$(oc get imagestreamtag -n "$NAMESPACE" -o json | \
    jq -r --arg is "$IMAGESTREAM" '.items[] | 
    select(.metadata.name | startswith($is + ":")) | 
    .metadata.name' | \
    sed "s/^$IMAGESTREAM://" | \
    sort)

if [ -z "$REMAINING_TAGS" ]; then
    print_warning "No tags remaining"
else
    echo "$REMAINING_TAGS" | while read -r tag; do
        echo "  - $tag"
    done
fi

print_success "Done!"

# Made with Bob
