# What is this package for?

Newly added types to the GraphQL schema will end up in a file called `models_gen.go` in this package. The file is not
under version control, and all models inside the generated file **must** be moved to their respective packages.

Generic types that does not necessarily belong to a specific package should be moved to the `model` package in the
`internal/graph/model` directory.
