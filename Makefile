LUA_FORMATTER_VERSION = 1.5.6
BIN_DIR := $(shell pwd)/bin
LUAFMT=$(BIN_DIR)/luafmt-$(LUA_FORMATTER_VERSION)

all: generate test check build helm-lint

helm-lint:
	helm lint --strict ./charts/fasit
	helm lint --strict ./charts/naisd

generate: generate-graphql generate-sql generate-feature-schema tester_spec

generate-sql: sqlc-vet
	go tool github.com/sqlc-dev/sqlc/cmd/sqlc generate
	$(MAKE) generate-mocks

generate-graphql:
	go tool github.com/99designs/gqlgen generate
	go tool mvdan.cc/gofumpt -w .

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
	go test -race -tags integration_test -cover ./...

unit-test:
	go test -cover ./...

sqlc-vet:
	go tool github.com/sqlc-dev/sqlc/cmd/sqlc vet

generate-mocks:
	go tool github.com/vektra/mockery/v2

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
	go tool honnef.co/go/tools/cmd/staticcheck ./...

vulncheck:
	go tool golang.org/x/vuln/cmd/govulncheck ./...

deadcode:
	go tool golang.org/x/tools/cmd/deadcode -test ./...

gosec:
	go tool github.com/securego/gosec/v2/cmd/gosec --exclude-generated -terse ./...

build: build-fasit build-naisd build-generate-schema build-setup-local-env

build-fasit:
	go build -o ./bin/fasit ./cmd/fasit/

build-naisd:
	go build -o ./bin/naisd ./cmd/naisd/

build-generate-schema:
	go build -o ./bin/generate_schema ./cmd/generate_schema/

build-setup-local-env:
	go build -o ./bin/setup_local_env ./cmd/setup_local_env/

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
