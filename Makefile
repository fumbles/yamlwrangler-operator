# Image URL to use all building/pushing image targets
# Use TAG environment variable or generate timestamp-based tag
TAG ?= v1.0.0-$(shell date +%Y%m%d%H%M%S)
IMG ?= default-route-openshift-image-registry.apps.sno.yamlwrangler.com/app-dashboard-operator/app-dashboard-operator:$(TAG)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

##@ Build

.PHONY: build
build: fmt vet ## Build manager binary.
	go build -o bin/manager main.go

.PHONY: run
run: fmt vet ## Run a controller from your host.
	go run ./main.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

.PHONY: podman-build
podman-build: ## Build podman image with the manager.
	podman build -t ${IMG} .

.PHONY: podman-push
podman-push: ## Push podman image with the manager.
	podman push ${IMG}

##@ Deployment

.PHONY: install
install: ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kubectl apply -f manifests/crd-dashboardappgroup.yaml

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f manifests/crd-dashboardappgroup.yaml

.PHONY: deploy
deploy: ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	kubectl apply -f manifests/deploy/

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f manifests/deploy/

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# Made with Bob
