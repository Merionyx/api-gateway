# Contributing

Thanks for your interest in this API Gateway project. This document explains how to work on the codebase and propose changes.

## Prerequisites

* **Go** — see `go.mod` for the required toolchain version (currently **Go 1.26.x**). Use the same or newer patch release.
* **Git**
* Optional: **Docker** — for integration tests and local stacks described in the Makefile
* Optional: **golangci-lint**, **buf**, **protoc** — only if you change code paths that use them (see Makefile targets)

## Getting started

```bash
git clone https://github.com/merionyx/api-gateway.git
cd api-gateway
go mod download
```

## Build & test

From the repository root:

| Goal                                                                      | Command                                |
| ------------------------------------------------------------------------- | -------------------------------------- |
| Unit tests only                                                           | `make test-unit`                       |
| Full test pipeline (unit + coverage gate + integration, needs Docker)     | `make test`                            |
| Coverage (local HTML report)                                              | `make test-coverage`                   |
| Coverage (CI-style gate)                                                  | `make test-coverage-ci`                |
| Integration tests only (Docker / etcd)                                    | `make test-integration`                |
| OIDC E2E subset (mock IdP + etcd)                                         | `make test-integration-oidc`           |
| Lint                                                                      | `make lint` (requires `golangci-lint`) |
| Format                                                                    | `make fmt`                             |
| Build binaries                                                            | `make build`                           |
| Build CLI (`agwctl`)                                                      | `make build-cli` → `./bin/agwctl`      |
| Build Docker images                                                       | `make docker-build`                    |
| Dev Docker Compose                                                        | `make docker-up-dev`                   |
| Docker Compose stack                                                      | `make docker-up`                       |

OpenAPI / codegen: after editing `apis/rest/api-server/openapi.yaml`, regenerate per `internal/api-server/gen/apiserver/doc.go` and `internal/cli/apiserver/client/doc.go` (`go generate` in those packages).

## Pull requests

1. **Open an issue first** for larger features or design changes, unless it’s a small fix (typos, obvious bugs).
2. **One logical change per PR** — easier to review and bisect.
3. **Tests** — add or update tests when behavior changes; keep `go test ./...` passing.
4. **CI** — PRs run GitHub Actions (lint, unit tests, etc.). Fix failures before merge.
5. **Commits** — write clear messages; follow any conventions the maintainers use in this repo.

### Before you raise a PR

> [!NOTE]
> Open pull requests from a **topic branch** (not your fork’s default branch you merge from, e.g. not `main`), so maintainers can push fixes to the PR branch when needed.

Before you send the PR, please run the following commands locally:

```sh
make tidy
make test-unit
make test-coverage-ci
make build
make fmt
make lint
```

For a final check before merge (requires Docker), also run `make test` or `make test-integration` as appropriate.

## Code review

Maintainers may request changes; discussion should stay respectful and on-topic (see `CODE_OF_CONDUCT.md`).

## Security

> [!CAUTION]
> Do **not** file public issues for security vulnerabilities. See [`SECURITY.md`](SECURITY.md).

## Questions

Use **GitHub Discussions** or **Issues** for questions that aren’t security-sensitive. If unsure, open an issue and ask.
