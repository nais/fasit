.PHONY: test integration-test local-with-auth local linux-build docker-build docker-push run-postgres-test stop-postgres-test install-sqlc
SQLC_VERSION ?= "v1.15.0"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
	GOBIN=$(shell go env GOPATH)/bin
else
	GOBIN=$(shell go env GOBIN)
endif

install-sqlc:
	go install github.com/kyleconroy/sqlc/cmd/sqlc@$(SQLC_VERSION)

generate-sql:
	$(GOBIN)/sqlc generate
	$(MAKE) mocks

generate-graphql:
	go run github.com/99designs/gqlgen generate

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

test:
	go test -cover ./...

integration-test:
	go test -tags integration_test -cover ./...

mocks:
	mockery --case underscore --name Repo --dir pkg/database/ --outpkg mocks --output pkg/database/mocks
	mockery --case underscore --name Querier --dir pkg/database/ --outpkg mocks --output pkg/database/mocks

generate-proto:
	mkdir -p pkg/provider/protogen
	protoc \
		-I schema/protobuf/ \
		./schema/protobuf/*.proto \
		--go_out=. \
		--go-grpc_out=.

generate-feature-schema:
	go run cmd/generate_schema/main.go

playground:
	cd cmd/mapping_playground && go run .
