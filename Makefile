# SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

EXTENSION_PREFIX            := gardener-extension
NAME                        := shoot-flux
REPO 						:= ghcr.io/23technologies
REPO_ROOT                   := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
HACK_DIR                    := $(REPO_ROOT)/hack
TAG                     := $(shell cat "$(REPO_ROOT)/VERSION")
LD_FLAGS                    := "-w $(shell $(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/get-build-ld-flags.sh k8s.io/component-base $(REPO_ROOT)/VERSION $(EXTENSION_PREFIX)-$(NAME))"
LEADER_ELECTION             := false
IGNORE_OPERATION_ANNOTATION := true
KUBECONFIG                  := dev/kubeconfig.yaml

WEBHOOK_CONFIG_PORT	:= 8444
WEBHOOK_CONFIG_URL	:= localhost:${WEBHOOK_CONFIG_PORT}
EXTENSION_NAMESPACE	:=

WEBHOOK_PARAM := --webhook-config-url=${WEBHOOK_CONFIG_URL}
ifeq (${WEBHOOK_CONFIG_MODE}, service)
  WEBHOOK_PARAM := --webhook-config-namespace=${EXTENSION_NAMESPACE}
endif


TOOLS_DIR := hack/tools
include $(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/tools.mk
include hack/tools.mk

.PHONY: start
start:
	@LEADER_ELECTION_NAMESPACE=garden GO111MODULE=on go run \
		-mod=vendor \
		-ldflags $(LD_FLAGS) \
		./cmd/$(EXTENSION_PREFIX)-$(NAME) \
		--kubeconfig=${KUBECONFIG} \
		--ignore-operation-annotation=$(IGNORE_OPERATION_ANNOTATION) \
		--leader-election=$(LEADER_ELECTION) \
		--webhook-config-server-host=localhost \
		--webhook-config-server-port=${WEBHOOK_CONFIG_PORT} \
		--gardener-version="v1.39.0"

.PHONY: debug
debug:
	@LEADER_ELECTION_NAMESPACE=garden GO111MODULE=on dlv debug\
		./cmd/$(EXTENSION_PREFIX)-$(NAME) -- \
		--kubeconfig=${KUBECONFIG} \
		--ignore-operation-annotation=$(IGNORE_OPERATION_ANNOTATION) \
		--leader-election=$(LEADER_ELECTION) \
		--webhook-config-server-host=localhost \
		--webhook-config-server-port=${WEBHOOK_CONFIG_PORT} \
		--gardener-version="v1.39.0"

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

.PHONY: install
install:
	@LD_FLAGS=$(LD_FLAGS) \
	$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/install.sh ./...

PUSH ?= false
.PHONY: images
images: $(KO)
	KO_DOCKER_REPO=$(REPO) $(KO) build --sbom none --base-import-paths -t $(TAG) --platform linux/amd64,linux/arm64 --push=$(PUSH) ./cmd/gardener-extension-shoot-flux

.PHONY: docker-images
docker-images:
	@docker build -t $(REPO)/$(NAME):$(TAG) -t $(REPO)/$(NAME):latest -f Dockerfile -m 6g --target $(EXTENSION_PREFIX)-$(NAME) .

.PHONY: controller-registration
controller-registration:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/generate-controller-registration.sh shoot-flux charts/gardener-extension-shoot-flux $(TAG) controller-registartion.yaml Extension:shoot-flux

#####################################################################
# Rules for verification, formatting, linting, testing and cleaning #
#####################################################################

.PHONY: revendor
revendor:
	@GO111MODULE=on go mod tidy
	@GO111MODULE=on go mod vendor
	@chmod +x $(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/*
	@chmod +x $(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/.ci/*
	@ln -sf ../vendor/github.com/gardener/gardener/hack/cherry-pick-pull.sh $(HACK_DIR)/cherry-pick-pull.sh

#	@$(REPO_ROOT)/hack/update-github-templates.sh

.PHONY: clean
clean:
	@$(shell find ./example -type f -name "controller-registration.yaml" -exec rm '{}' \;)
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/clean.sh ./cmd/... ./pkg/...

.PHONY: check-generate
check-generate:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/check-generate.sh $(REPO_ROOT)

.PHONY: check-docforge
check-docforge: $(DOCFORGE)
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/check-docforge.sh $(REPO_ROOT) $(REPO_ROOT)/.docforge/manifest.yaml ".docforge/;docs/" "gardener-extension-provider-openstack" false

.PHONY: check
check: $(GOIMPORTS) $(GOLANGCI_LINT)
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/check.sh --golangci-lint-config=./.golangci.yaml ./cmd/... ./pkg/...
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/check-charts.sh ./charts

.PHONY: generate
generate: $(CONTROLLER_GEN) $(GEN_CRD_API_REFERENCE_DOCS) $(HELM) $(MOCKGEN) $(YQ)
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/generate.sh ./charts/... ./cmd/... ./example/... ./pkg/...
	$(MAKE) format

.PHONY: format
format: $(GOIMPORTS) $(GOIMPORTSREVISER)
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/format.sh ./cmd ./pkg

.PHONY: test
test:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/test.sh ./cmd/... ./pkg/...

.PHONY: test-cov
test-cov:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/test-cover.sh ./cmd/... ./pkg/...

.PHONY: test-clean
test-clean:
	@$(REPO_ROOT)/vendor/github.com/gardener/gardener/hack/test-cover-clean.sh

.PHONY: verify
verify: check format test

.PHONY: verify-extended
verify-extended: check-generate check format test-cov test-clean