#!/usr/bin/env bash
#MISE description="Generate all code"

mise run generate:graphql
mise run generate:proto
mise run generate:sqlc
mise run generate:mocks
mise run generate:tester-spec
