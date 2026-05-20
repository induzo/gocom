# triple-a/gocom

[![security scanner](https://github.com/triple-a/gocom/actions/workflows/sec-scanner.yml/badge.svg)](https://github.com/triple-a/gocom/actions/workflows/sec-scanner.yml) [![linter](https://github.com/triple-a/gocom/actions/workflows/linter.yml/badge.svg)](https://github.com/triple-a/gocom/actions/workflows/linter.yml) [![tests](https://github.com/triple-a/gocom/actions/workflows/tests.yml/badge.svg)](https://github.com/triple-a/gocom/actions/workflows/tests.yml) [![conv commits checker](https://github.com/triple-a/gocom/actions/workflows/conv-commits-checker.yml/badge.svg)](https://github.com/triple-a/gocom/actions/workflows/conv-commits-checker.yml) [![codecov](https://codecov.io/gh/triple-a/gocom/branch/main/graph/badge.svg?token=UBWDRLOYDU)](https://codecov.io/gh/triple-a/gocom)

common golang packages

## Current modules

| module                                                             | benchmarks                                                         | latest version | report                                                                                                                                                                                               | docs                                                                                                                                                                                |
| ------------------------------------------------------------------ | ------------------------------------------------------------------ | -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [database/pginit](database/pginit)                                 | [benches](https://triple-a.github.io/gocom/database/pginit)          | 2.2.34          | [![Go Report Card](https://goreportcard.com/badge/github.com/triple-a/gocom/database/pginit)](https://goreportcard.com/report/github.com/triple-a/gocom/database/pginit)                                 | [![Go Reference](https://pkg.go.dev/badge/github.com/triple-a/gocom/database/pginit.svg)](https://pkg.go.dev/github.com/triple-a/gocom/database/pginit)                                 |
| [database/pgx-slog](database/pgx-slog)                             | [benches](https://triple-a.github.io/gocom/database/pgx-slog)        | 1.0.38          | [![Go Report Card](https://goreportcard.com/badge/github.com/triple-a/gocom/database/pgx-slog)](https://goreportcard.com/report/github.com/triple-a/gocom/database/pgx-slog)                             | [![Go Reference](https://pkg.go.dev/badge/github.com/triple-a/gocom/database/pgx-slog.svg)](https://pkg.go.dev/github.com/triple-a/gocom/database/pgx-slog)                             |
| [http/health](http/health)                                         | [benches](https://triple-a.github.io/gocom/http/health)              | 1.2.0          | [![Go Report Card](https://goreportcard.com/badge/github.com/triple-a/gocom/http/health)](https://goreportcard.com/report/github.com/triple-a/gocom/http/health)                                         | [![Go Reference](https://pkg.go.dev/badge/github.com/triple-a/gocom/http/health.svg)](https://pkg.go.dev/github.com/triple-a/gocom/http/health)                                         |
| [http/middleware/valkeydempotency](http/middleware/valkeydempotency) | [benches](https://github.com/triple-a/gocom/http/middleware/valkeydempotency)             | 0.4.9         | [![Go Report Card](https://goreportcard.com/badge/github.com/triple-a/gocom/http/middleware/valkeydempotency)](https://goreportcard.com/report/github.com/triple-a/gocom/http/middleware/valkeydempotency) | [![Go Reference](https://pkg.go.dev/badge/github.com/triple-a/gocom/http/middleware/valkeydempotency.svg)](https://pkg.go.dev/github.com/triple-a/gocom/http/middleware/valkeydempotency) |
| [http/middleware/idempotency](http/middleware/idempotency) | [benches](https://github.com/triple-a/gocom/http/middleware/idempotency)             | 0.10.0         | [![Go Report Card](https://goreportcard.com/badge/github.com/triple-a/gocom/http/middleware/idempotency)](https://goreportcard.com/report/github.com/triple-a/gocom/http/middleware/idempotency) | [![Go Reference](https://pkg.go.dev/badge/github.com/triple-a/gocom/http/middleware/idempotency.svg)](https://pkg.go.dev/github.com/triple-a/gocom/http/middleware/idempotency) |
| [http/middleware/writablecontext](http/middleware/writablecontext) | [benches](https://github.com/triple-a/gocom/http/middleware/writablecontext) | 0.2.0          | [![Go Report Card](https://goreportcard.com/badge/github.com/triple-a/gocom/http/middleware/writablecontext)](https://goreportcard.com/report/github.com/triple-a/gocom/http/middleware/writablecontext) | [![Go Reference](https://pkg.go.dev/badge/github.com/triple-a/gocom/http/middleware/writablecontext.svg)](https://pkg.go.dev/github.com/triple-a/gocom/http/middleware/writablecontext) |
| [shutdown](shutdown)                                               | [benches](https://triple-a.github.io/gocom/shutdown)                 | 1.3.5         | [![Go Report Card](https://goreportcard.com/badge/github.com/triple-a/gocom/shutdown)](https://goreportcard.com/report/github.com/triple-a/gocom/shutdown)                                               | [![Go Reference](https://pkg.go.dev/badge/github.com/triple-a/gocom/shutdown.svg)](https://pkg.go.dev/github.com/triple-a/gocom/shutdown)                                               |

## How to use any of these private modules

Force the use of ssh instead of https for git:

```bash
git config --global --add url."git@github.com:".insteadOf "https://github.com/"
```

Allow internal repositories under a private company, simply add this line to your .zshrc or other, accordingly:

```bash
export GOPRIVATE="github.com/triple-a/*"
```

For private modules, also set the checksum exclusion for the same namespace:

```bash
export GONOSUMDB="github.com/triple-a/*"
```

## Using from auberun

Consume these modules directly by versioned module tags:

```bash
go get github.com/triple-a/gocom/http/health@v1.2.0
go get github.com/triple-a/gocom/http/middleware/idempotency@v0.10.0
go get github.com/triple-a/gocom/database/pginit/v2@v2.2.34
```

If auberun uses CI, configure `GOPRIVATE` (and optionally `GONOSUMDB`) in the CI environment, and ensure git can authenticate to private GitHub repos.

## How to add a new module

Let's take an example of an opentelemetry module.

- Make sure the module is fully tested (at least 95% coverage, try to reach 100%), linted
- Create a branch feat/opentelemetry
- Copy in the right folder (that's quite subjective), in our case, ./monitoring/otelinit
- Add it to the workspace

```bash
    go work use ./monitoring/otelinit
```

- Add your file, commit your files (respecting conventional commits) and tag the commit properly, according to a semantic versioning

```bash
    git add ./monitoring/otelinit
    git commit -m "feat: add monitoring opentelemetry module" ./monitoring/otelinit
    git tag "monitoring/otelinit/v1.0.0"
```

- Create a pull request
- Wait for review

## Dependency graph

### Depends on database/pgx-slog

- database/pginit

### Depends on http/middleware/idempotency

- http/middleware/valkeydempotency

## Contribution

```bash
pre-commit install -t commit-msg -t pre-commit -t pre-push
```
