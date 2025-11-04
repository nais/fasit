helm-lint:
	mise run check:helm

generate-graphql:
	mise run generate:graphql

generate-sql:
	mise run generate:sqlc

generate-feature-schema:
	mise run generate:feature-schema

generate-mocks:
	mise run generate:mocks

generate-proto:
	mise run generate:proto

setup:
	mise run local:setup

local:
	mise run local:fasit

local-naisd:
	mise run local:naisd

local-naisd-failing:
	mise run local:naisd-failing

local-naisd-management:
	mise run local:naisd-management

local-naisd-management-failing:
	mise run local:naisd-management-failing

test:
	mise run test

unit-test:
	mise run test:unit

release-naisd:
	mise run release:naisd

staticcheck:
	mise run check:staticcheck

vulncheck:
	mise run check:govulncheck

deadcode:
	mise run check:deadcode

gosec:
	mise run check:gosec

build-fasit:
	mise run build:fasit

build-naisd:
	mise run build:naisd

build-generate-schema:
	mise run build:generate-schema

build-setup-local-env:
	mise run build:setup-local-env

integration_test_ui:
	mise run test:integration

tester_spec:
	mise run generate:tester-spec

prettier:
	mise run fmt:prettier

fmt:
	mise run fmt