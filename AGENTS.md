# AGENTS.md

## Scope

These instructions apply to the entire repository. More specific `AGENTS.md`
files in subdirectories override them for files in those directories.

## Project overview

Harmony is a Go blockchain node implementation. The main executables are the
Harmony node and bootnode. The repository also contains consensus, networking,
staking, RPC, Rosetta, and local-network test code.

The project uses Go 1.24.2. BLS support comes from the official
`github.com/herumi/bls-eth-go-binary` Go module with prebuilt static libraries;
separate MCL/BLS checkouts and custom native-library build flags are not needed.

## Working rules

- Keep changes focused on the requested task; do not modify unrelated files.
- Preserve existing public behavior unless the task explicitly changes it.
- Follow Effective Go and the conventions already used in the edited package.
- Format Go files with `gofmt` and `goimports`.
- Add or update tests for behavioral changes and bug fixes.
- Do not edit generated files directly. Change their source and use the
  repository's generation command.
- Never commit secrets, validator keys, local databases, logs, build outputs,
  or files created by localnet runs.
- Treat consensus, cryptography, staking, slashing, and state-transition
  changes as security-sensitive. Check boundary conditions and compatibility
  carefully.

## Useful commands

Run the narrowest relevant checks first:

```bash
go test ./path/to/changed/package
go test -run TestName ./path/to/changed/package
go vet ./path/to/changed/package
```

Repository-level commands:

```bash
make exe          # build the Harmony and bootnode executables
make go-vet       # run go vet for all packages
make go-test      # run all Go tests with vet and the race detector
make test-go      # run the Docker-based Go CI checks
make protofiles   # regenerate protobuf-derived Go code
make debug        # build and start a local development network
make debug-kill   # stop the local development network
```

Before reporting completion:

1. Run `gofmt`/`goimports` on changed Go files.
2. Run targeted tests for the changed packages.
3. Run broader checks when practical and state clearly which checks were not
   run or could not run.
4. Review `git diff --check` and the final diff for accidental changes.

## Build notes

- Prefer the Makefile and repository scripts over ad hoc build commands.
- Use `go mod download` or `make libs` to fetch the BLS dependency. Do not
  reintroduce sibling MCL/BLS checkouts or the removed native build-flag setup.
- Full test and localnet commands may require Docker, native libraries, open
  ports, and substantially more time than targeted package tests.
- Do not run cleanup targets or remove localnet data unless the user explicitly
  asks for cleanup.

## Documentation and pull requests

- Update documentation when configuration, commands, APIs, or operator-visible
  behavior changes.
- In the final report, summarize the change and list the exact validation
  commands and results.
- Pull request descriptions should include a `[Test]` section describing the
  checks performed.
