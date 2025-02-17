.PHONY: test integration-test local-with-auth local linux-build docker-build docker-push run-postgres-test stop-postgres-test

PROTOC = $(shell which protoc)
SQLC = go run github.com/sqlc-dev/sqlc/cmd/sqlc
MOCKERY = go run github.com/vektra/mockery/v2

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
	GOBIN=$(shell go env GOPATH)/bin
else
	GOBIN=$(shell go env GOBIN)
endif

generate-sql: sqlc-vet
	$(SQLC) generate
	$(MAKE) mocks

generate-graphql:
	go run github.com/99designs/gqlgen generate
	go run mvdan.cc/gofumpt@latest -w .

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

test: sqlc-vet
	go test -cover ./...

integration-test: sqlc-vet
	go test -tags integration_test -cover ./...

sqlc-vet:
	$(SQLC) vet


mocks:
	$(MOCKERY) --case underscore --name Repo --dir pkg/database/ --outpkg mocks --output pkg/database/mocks --with-expecter
	$(MOCKERY) --case underscore --name Querier --dir pkg/database/ --outpkg mocks --output pkg/database/mocks --with-expecter
	$(MOCKERY) --case underscore --name Upgrader --dir pkg/upgrader/ --outpkg mocks --output pkg/upgrader/mocks --with-expecter

install-protobuf-go:
	go install google.golang.org/protobuf/cmd/protoc-gen-go
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc

generate-proto: install-protobuf-go
	mkdir -p pkg/provider/protogen
	PATH="${PATH}:$(shell go env GOPATH)/bin" ${PROTOC} \
		-I schema/protobuf/ \
		./schema/protobuf/*.proto \
		--go_out=. \
		--go-grpc_out=.

generate-feature-schema:
	go run cmd/generate_schema/main.go

playground:
	echo "Playground has moved to Fasit: https://fasit.nais.io/playground"

release-naisd:
	./hack/release-naisd.sh

check: staticcheck vulncheck deadcode

staticcheck:
	go tool honnef.co/go/tools/cmd/staticcheck ./...

vulncheck:
	go tool golang.org/x/vuln/cmd/govulncheck ./...

deadcode:
	go tool golang.org/x/tools/cmd/deadcode -test ./...
