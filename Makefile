# ###################################################################### #
#  kuspace - developer Makefile                                           #
#                                                                         #
#  make            list targets                                           #
#  make setup      first-time setup (submodule, secrets, compose .env)    #
#  make up         build + start the docker-compose stack                 #
#  make smoke      end-to-end test against the running stack              #
#  make check      fmt + vet + unit tests (CI: make ci, with -race)       #
#                                                                         #
#  Kubernetes deployment lives in scripts/kuspacectl.go (make k8s-*).     #
# ###################################################################### #

SHELL        := /bin/bash
.DEFAULT_GOAL := help

# data/ (runtime data, root-owned dirs from containers) has its own go.mod,
# so ./... never walks into it; third_party/minioth is its own module too
PKGS         := ./...
SERVICES     := uspace frontapp wss
BIN          := bin

COMPOSE_DIR  := deployments/docker-compose
COMPOSE      := docker compose --project-directory $(COMPOSE_DIR) -f $(COMPOSE_DIR)/docker-compose.yml
SECRETS      := configs/secrets.env
MINIOTH_DIR  := third_party/minioth

.PHONY: help setup submodule secrets \
        build $(addprefix build-,$(SERVICES)) build-minioth run-% \
        fmt vet lint test test-race test-js check ci tidy \
        up down restart logs ps smoke images \
        k8s-build k8s-push k8s-deploy k8s-destroy \
        api-docs code-docs clean

help: ## list targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_%-]+:.*## / {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# --------------------------------------------------------------- setup
setup: submodule secrets ## first-time setup: minioth submodule, secrets, compose .env

submodule: ## fetch the pinned minioth submodule
	git submodule update --init --recursive

secrets: ## create configs/secrets.env with fresh random values (never overwrites)
	@if [ -e $(SECRETS) ]; then echo "$(SECRETS) exists, leaving it alone"; else \
	  umask 077; svc=$$(openssl rand -hex 32); \
	  sed -e "s|^JWT_SECRET_KEY=.*|JWT_SECRET_KEY=$$(openssl rand -hex 64)|" \
	      -e "s|^JWT_REFRESH_SECRET_KEY=.*|JWT_REFRESH_SECRET_KEY=$$(openssl rand -hex 64)|" \
	      -e "s|^SERVICE_SECRET_KEY=.*|SERVICE_SECRET_KEY=$$svc|" \
	      -e "s|^MINIOTH_SERVICE_SECRETS=.*|MINIOTH_SERVICE_SECRETS=uspace:$$svc,frontapp:$$svc,wss:$$svc|" \
	      -e "s|^MINIOTH_SECRET_KEY=.*|MINIOTH_SECRET_KEY=$$(openssl rand -hex 16)|" \
	      -e "s|^MINIO_SECRET_KEY=.*|MINIO_SECRET_KEY=$$(openssl rand -hex 16)|" \
	      -e "s|^FSL_SECRET_KEY=.*|FSL_SECRET_KEY=$$(openssl rand -hex 16)|" \
	      configs/secrets.env.example > $(SECRETS); \
	  echo "created $(SECRETS)"; fi
	@ln -sfn ../../$(SECRETS) $(COMPOSE_DIR)/.env

# --------------------------------------------------------------- build
build: $(addprefix build-,$(SERVICES)) build-minioth ## build all service binaries into bin/

build-%: ## build one service binary, e.g. make build-uspace
	go build -o $(BIN)/$* ./cmd/$*

build-minioth: ## build minioth from the pinned submodule
	cd $(MINIOTH_DIR) && go build -o ../../$(BIN)/minioth ./cmd/minioth

run-%: build-% ## build and run one service on the host, e.g. make run-frontapp
	$(BIN)/$*

# --------------------------------------------------------------- quality
fmt: ## gofmt check (lists files that need formatting)
	@out=$$(gofmt -l cmd internal pkg scripts); if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

vet: ## go vet
	go vet $(PKGS)

lint: ## golangci-lint (config: .golangci-lint.yaml)
	golangci-lint run -c .golangci-lint.yaml $(PKGS)

test: ## unit tests
	go test $(PKGS)

test-race: ## unit tests with the race detector
	go test -race $(PKGS)

test-js: ## browser JS tests (node:test; languages without an interpreter are skipped)
	node --test web/tests/

check: fmt vet test ## fmt + vet + unit tests

ci: fmt vet test-race test-js ## what CI runs (.github/workflows/ci.yml)

tidy: ## go mod tidy
	go mod tidy

# --------------------------------------------------------------- docker compose
# images are built one at a time: the services link cgo (sqlite, duckdb) and
# building them in parallel runs out of memory on 8 GB machines
images: ## build all compose images, sequentially
	@for s in minioth wss uspace frontapp; do echo "building $$s"; $(COMPOSE) build $$s || exit 1; done

up: secrets images ## build and start the compose stack
	$(COMPOSE) up -d
	@$(COMPOSE) ps

down: ## stop the compose stack (keeps volumes)
	$(COMPOSE) down

restart: ## restart the compose services without rebuilding
	$(COMPOSE) restart

logs: ## follow compose logs (make logs S=uspace for one service)
	$(COMPOSE) logs -f $(S)

ps: ## compose service status
	$(COMPOSE) ps

smoke: ## end-to-end smoke test against the running compose stack
	scripts/smoke.sh

# --------------------------------------------------------------- kubernetes
k8s-build: ## build all images (kuspacectl)
	go run scripts/kuspacectl.go -build

k8s-push: ## build and push images
	go run scripts/kuspacectl.go -build -push

k8s-deploy: ## deploy manifests, config maps and secrets to the cluster
	go run scripts/kuspacectl.go -deploy

k8s-destroy: ## delete the kuspace namespace
	go run scripts/kuspacectl.go -destroy

# --------------------------------------------------------------- docs
# go install github.com/swaggo/swag/cmd/swag@latest
# go install github.com/go101/golds@latest
api-docs: ## regenerate swagger docs (uspace, fslite)
	swag init -g internal/uspace/api.go -o api/uspace --instanceName uspacedocs --exclude pkg/fslite --parseDependency --parseInternal
	swag init -g pkg/fslite/fslite_server.go -o api/fslite --instanceName fslitedocs --exclude internal/uspace --parseDependency --parseInternal

code-docs: ## generate browsable code docs into docs/
	golds -gen -dir docs/fslite -compact -wdpkgs-listing solo ./pkg/fslite/
	golds -gen -dir docs/uspace -compact -wdpkgs-listing solo ./internal/uspace/

# --------------------------------------------------------------- cleanup
clean: ## remove built binaries and generated code docs
	rm -rf $(BIN) docs/fslite docs/uspace
