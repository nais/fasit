.PHONY: test integration-test local-with-auth local linux-build docker-build docker-push run-postgres-test stop-postgres-test install-sqlc 
SQLC_VERSION ?= "v1.12.0"

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

generate-graphql:
	go get -d github.com/99designs/gqlgen@latest && go run github.com/99designs/gqlgen generate

local:
	go run ./cmd/c3po \
	--bind-address=127.0.0.1:8080 \
	--log-level=debug
