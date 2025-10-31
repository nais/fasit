LUA_FORMATTER_VERSION = 1.5.6
BIN_DIR := $(shell pwd)/bin
LUAFMT=$(BIN_DIR)/luafmt-$(LUA_FORMATTER_VERSION)

all: generate fmt test check build helm-lint

helm-lint:
	mise run check:helm

generate: generate-graphql generate-sql generate-feature-schema tester_spec

generate-sql:
	mise run generate:sqlc

generate-graphql:
	mise run generate:graphql

setup:
	go run cmd/setup_local_env/main.go

local:
	PUBSUB_EMULATOR_HOST=localhost:8086 go run ./cmd/fasit \
	--bind-address=127.0.0.1:8080 \
	--grpc-bind-address=127.0.0.1:4444 \
	--log-level=debug \
	--insecure-skip-proxy \
	--insecure-skip-token-check

local-naisd:
	PUBSUB_EMULATOR_HOST=localhost:8086 go run ./cmd/naisd \
	--env-project-id local-test-partner-dev \
	--nais-project-id nais-local-dev \
	--tenant-name test-partner \
	--env dev \
	--log-level=debug

local-naisd-failing:
	PUBSUB_EMULATOR_HOST=localhost:8086 go run ./cmd/naisd \
	--env-project-id local-test-partner-dev \
	--nais-project-id nais-local-dev \
	--tenant-name test-partner \
	--env dev \
	--log-level=debug \
	--mock-failing=true

local-naisd-management:
	PUBSUB_EMULATOR_HOST=localhost:8086 go run ./cmd/naisd \
	--env-project-id local-test-partner-management \
	--nais-project-id nais-local-dev \
	--tenant-name test-partner \
	--env management \
	--management=true \
	--log-level=debug

local-naisd-management-failing:
	PUBSUB_EMULATOR_HOST=localhost:8086 go run ./cmd/naisd \
	--env-project-id local-test-partner-management \
	--nais-project-id nais-local-dev \
	--tenant-name test-partner \
	--env management \
	--management=true \
	--log-level=debug \
	--mock-failing=true

test:
	mise run test

unit-test:
	mise run test:unit

generate-mocks:
	mise run generate:mocks

generate-proto:
	mkdir -p internal/provider/protogen
	protoc \
		-I schema/protobuf/ \
		./schema/protobuf/*.proto \
		--go_out=. \
		--go-grpc_out=.

generate-feature-schema:
	go run cmd/generate_schema/main.go

release-naisd:
	./hack/release-naisd.sh

check: staticcheck vulncheck deadcode gosec

staticcheck:
	mise run check:staticcheck

vulncheck:
	mise run check:govulncheck

deadcode:
	mise run check:deadcode

gosec:
	mise run check:gosec

build: build-fasit build-naisd build-generate-schema build-setup-local-env

build-fasit:
	mise run build:fasit

build-naisd:
	mise run build:naisd

build-generate-schema:
	mise run build:generate-schema

build-setup-local-env:
	mise run build:setup-local-env

integration_test_ui:
	go run ./cmd/tester_run --ui

tester_spec:
	go run ./cmd/tester_spec

prettier:
	npm install
	npx prettier --write .

fmt: prettier install-lua-formatter
	go tool mvdan.cc/gofumpt -w ./
	$(LUAFMT)/bin/CodeFormat format -w . --ignores-file ".gitignore" -c ./integration_tests/.editorconfig

LUA_FORMATTER_URL := https://github.com/CppCXY/EmmyLuaCodeStyle/releases/download/$(LUA_FORMATTER_VERSION)
OS := $(shell uname -s)
ARCH := $(shell uname -m)

ifeq ($(OS), Darwin)
  ifeq ($(ARCH), x86_64)
    LUA_FORMATTER_FILE := darwin-x64
  else
    ifeq ($(ARCH), arm64)
      LUA_FORMATTER_FILE := darwin-arm64
    else
      $(error Unsupported architecture: $(ARCH) on macOS)
    endif
  endif
else ifeq ($(OS), Linux)
  ifeq ($(ARCH), x86_64)
    LUA_FORMATTER_FILE := linux-x64
  else
    ifeq ($(ARCH), aarch64)
      LUA_FORMATTER_FILE := linux-aarch64
    else
      $(error Unsupported architecture: $(ARCH) on Linux)
    endif
  endif
else
  $(error Unsupported OS: $(OS))
endif

install-lua-formatter: $(LUAFMT)
$(LUAFMT):
	@mkdir -p $(LUAFMT)
	@curl -L $(LUA_FORMATTER_URL)/$(LUA_FORMATTER_FILE).tar.gz -o /tmp/luafmt.tar.gz
	@tar -xzf /tmp/luafmt.tar.gz -C $(LUAFMT)
	@rm /tmp/luafmt.tar.gz
	@mv $(LUAFMT)/$(LUA_FORMATTER_FILE)/* $(LUAFMT)/
	@rmdir $(LUAFMT)/$(LUA_FORMATTER_FILE)
