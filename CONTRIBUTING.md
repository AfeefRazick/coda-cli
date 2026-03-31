# Contributing to coda-cli

Thank you for your interest in contributing!

## Getting started

```sh
git clone https://github.com/AfeefRazick/coda-cli
cd coda-cli
go mod tidy
make build
./bin/coda --help
```

Requirements: Go 1.23+

## Running tests

```sh
make test
```

## Code style

- `go vet ./...` must pass
- Follow standard Go conventions
- New commands should include an `Example` field in their `cobra.Command`

## Adding a new command

Commands live in `internal/cmd/`. Each resource (docs, pages, rows, etc.) has its own file. Add new subcommands to the relevant file or create a new one for a new resource group. Register the top-level command in `internal/cmd/root.go`.

## Submitting changes

1. Fork the repo and create a branch
2. Make your changes
3. Run `make test` and `go vet ./...`
4. Open a pull request against `main`

## Issues

Please search existing issues before opening a new one.
