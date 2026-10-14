# Stellar

An experimental statically typed language. See [docs/design.md](docs/design.md)
for the specification and [docs/status.md](docs/status.md) for where the
project stands.

```sh
go test ./...                                  # all tests
go test ./internal/parser -update              # regenerate parser golden files
go test ./internal/parser -run '^$' -fuzz FuzzParseFile -fuzztime 30s
go run ./cmd/stellar parse file.st             # parse and print canonical form
go run ./cmd/stellar check file.st             # type-check
go run ./cmd/stellar run file.st               # type-check and run
go test ./internal/interp -update              # regenerate interpreter golden outputs
```
