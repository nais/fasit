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

setup:
	go run cmd/setup_pubsub/main.go

local:
	SPANNER_EMULATOR_HOST=localhost:9010 PUBSUB_EMULATOR_HOST=localhost:8085 go run ./cmd/fasit \
	--bind-address=127.0.0.1:8080 \
	--log-level=debug

create-gcloud-config:
	gcloud config configurations create fasit ;\
  gcloud config set auth/disable_credentials true ;\
  gcloud config set project nais-local-dev ;\
  gcloud config set api_endpoint_overrides/spanner http://localhost:9020/ ;\
	gcloud config configurations list

spanner-setup:
	gcloud config configurations activate fasit && \
	gcloud spanner instances create fasit \
   --config=emulator-config --description="Fasit Instance" --nodes=1 && \
	gcloud spanner databases create fasit --instance=fasit
