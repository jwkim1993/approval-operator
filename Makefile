SDK	= operator-sdk

REGISTRY      ?= tmaxcloudck
VERSION       ?= 0.0.1

PACKAGE_NAME  = approval-operator
OPERATOR_IMG  = $(REGISTRY)/$(PACKAGE_NAME):$(VERSION)

WATCHER_NAME  = approval-watcher
WATCHER_IMG   = $(REGISTRY)/$(WATCHER_NAME):$(VERSION)

BIN = ./build/_output/bin

BUILD_FLAG  = -gcflags all=-trimpath=/home/sunghyun/dev -asmflags all=-trimpath=/home/sunghyun/dev

.PHONY: all
all: test gen build push

.PHONY: clean
clean:
	rm -rf $(BIN)

.PHONY: gen
gen:
	$(SDK) generate k8s
	$(SDK) generate crds --crd-version v1


.PHONY: build build-operator build-watcher
build: build-operator build-watcher

build-operator:
	$(SDK) build $(OPERATOR_IMG)

build-watcher:


.PHONY: push push-operator push-watcher
push: push-operator push-watcher

push-operator:
	docker push $(OPERATOR_IMG)

push-watcher:


.PHONY: test test-gen save-sha-gen compare-sha-gen test-verify save-sha-mod compare-sha-mod verify test-unit test-lint
test: test-gen test-verify test-unit test-lint

test-gen: save-sha-gen gen compare-sha-gen

save-sha-gen:
	$(eval CRDSHA=$(shell sha512sum deploy/crds/tmax.io_approvals_crd.yaml))
	$(eval GENSHA=$(shell sha512sum pkg/apis/tmax/v1/zz_generated.deepcopy.go))

compare-sha-gen:
	$(eval CRDSHA_AFTER=$(shell sha512sum deploy/crds/tmax.io_approvals_crd.yaml))
	$(eval GENSHA_AFTER=$(shell sha512sum pkg/apis/tmax/v1/zz_generated.deepcopy.go))
	@if [ "${CRDSHA_AFTER}" = "${CRDSHA}" ]; then echo "deploy/crds/tmax.io_approvals_crd.yaml is not changed"; else echo "deploy/crds/tmax.io_approvals_crd.yaml file is changed"; exit 1; fi
	@if [ "${GENSHA_AFTER}" = "${GENSHA}" ]; then echo "zz_generated.deepcopy.go is not changed"; else echo "zz_generated.deepcopy.go file is changed"; exit 1; fi

test-verify: save-sha-mod verify compare-sha-mod

save-sha-mod:
	$(eval MODSHA=$(shell sha512sum go.mod))
	$(eval SUMSHA=$(shell sha512sum go.sum))

verify:
	go mod verify

compare-sha-mod:
	$(eval MODSHA_AFTER=$(shell sha512sum go.mod))
	$(eval SUMSHA_AFTER=$(shell sha512sum go.sum))
	@if [ "${MODSHA_AFTER}" = "${MODSHA}" ]; then echo "go.mod is not changed"; else echo "go.mod file is changed"; exit 1; fi
	@if [ "${SUMSHA_AFTER}" = "${SUMSHA}" ]; then echo "go.sum is not changed"; else echo "go.sum file is changed"; exit 1; fi

test-unit:
	go test -v ./pkg/...

test-lint:
	golangci-lint run ./... -v -E gofmt --timeout 1h0m0s


.PHONY: run-local deploy
run-local:
	$(SDK) run --local --watch-namespace=""

deploy:
	kubectl apply -f deploy/service_account.yaml
	kubectl apply -f deploy/role.yaml
	kubectl apply -f deploy/role_binding.yaml
	kubectl apply -f deploy/crds/tmax.io_approvals_crd.yaml
	kubectl apply -f deploy/service.yaml
	kubectl apply -f deploy/validating_webhook_config.yaml
	kubectl apply -f deploy/operator.yaml
