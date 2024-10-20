# induzo/gocom

[![security scanner](https://github.com/induzo/gocom/actions/workflows/sec-scanner.yml/badge.svg)](https://github.com/induzo/gocom/actions/workflows/sec-scanner.yml) [![linter](https://github.com/induzo/gocom/actions/workflows/linter.yml/badge.svg)](https://github.com/induzo/gocom/actions/workflows/linter.yml) [![tests](https://github.com/induzo/gocom/actions/workflows/tests.yml/badge.svg)](https://github.com/induzo/gocom/actions/workflows/tests.yml) [![conv commits checker](https://github.com/induzo/gocom/actions/workflows/conv-commits-checker.yml/badge.svg)](https://github.com/induzo/gocom/actions/workflows/conv-commits-checker.yml) [![codecov](https://codecov.io/gh/induzo/gocom/branch/main/graph/badge.svg?token=UBWDRLOYDU)](https://codecov.io/gh/induzo/gocom)

common golang packages

## Current modules

| module                                                             | benchmarks                                                         | latest version | report                                                                                                                                                                                               | docs                                                                                                                                                                                |
| ------------------------------------------------------------------ | ------------------------------------------------------------------ | -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [database/pginit](database/pginit)                                 | [benches](https://induzo.github.io/gocom/database/pginit)          | 2.2.9          | [![Go Report Card](https://goreportcard.com/badge/github.com/induzo/gocom/database/pginit)](https://goreportcard.com/report/github.com/induzo/gocom/database/pginit)                                 | [![Go Reference](https://pkg.go.dev/badge/github.com/induzo/gocom/database/pginit.svg)](https://pkg.go.dev/github.com/induzo/gocom/database/pginit)                                 |
| [database/pgx-slog](database/pgx-slog)                             | [benches](https://induzo.github.io/gocom/database/pgx-slog)        | 1.0.15         | [![Go Report Card](https://goreportcard.com/badge/github.com/induzo/gocom/database/pgx-slog)](https://goreportcard.com/report/github.com/induzo/gocom/database/pgx-slog)                             | [![Go Reference](https://pkg.go.dev/badge/github.com/induzo/gocom/database/pgx-slog.svg)](https://pkg.go.dev/github.com/induzo/gocom/database/pgx-slog)                             |
| [database/redisinit](database/redisinit)                           | [benches](https://induzo.github.io/gocom/database/redisinit)       | 1.0.9          | [![Go Report Card](https://goreportcard.com/badge/github.com/induzo/gocom/database/redisinit)](https://goreportcard.com/report/github.com/induzo/gocom/database/redisinit)                           | [![Go Reference](https://pkg.go.dev/badge/github.com/induzo/gocom/database/redisinit.svg)](https://pkg.go.dev/github.com/induzo/gocom/database/redisinit)                           |
| [http/health](http/health)                                         | [benches](https://induzo.github.io/gocom/http/health)              | 1.1.9          | [![Go Report Card](https://goreportcard.com/badge/github.com/induzo/gocom/http/health)](https://goreportcard.com/report/github.com/induzo/gocom/http/health)                                         | [![Go Reference](https://pkg.go.dev/badge/github.com/induzo/gocom/http/health.svg)](https://pkg.go.dev/github.com/induzo/gocom/http/health)                                         |
| [http/middleware/writablecontext](http/middleware/writablecontext) | [benches](github.com/induzo/gocom/http/middleware/writablecontext) | 0.1.6          | [![Go Report Card](https://goreportcard.com/badge/github.com/induzo/gocom/http/middleware/writablecontext)](https://goreportcard.com/report/github.com/induzo/gocom/http/middleware/writablecontext) | [![Go Reference](https://pkg.go.dev/badge/github.com/induzo/gocom/http/middleware/writablecontext.svg)](https://pkg.go.dev/github.com/induzo/gocom/http/middleware/writablecontext) |
| [shutdown](shutdown)                                               | [benches](https://induzo.github.io/gocom/shutdown)                 | 1.2.5          | [![Go Report Card](https://goreportcard.com/badge/github.com/induzo/gocom/shutdown)](https://goreportcard.com/report/github.com/induzo/gocom/shutdown)                                               | [![Go Reference](https://pkg.go.dev/badge/github.com/induzo/gocom/shutdown.svg)](https://pkg.go.dev/github.com/induzo/gocom/shutdown)                                               |

## How to use any of these private modules

Force the use of ssh instead of https for git:

```bash
git config --global --add url."git@github.com:".insteadOf "https://github.com/"
```

Allow internal repositories under a private company, simply add this line to your .zshrc or other, accordingly:

```bash
export GOPRIVATE="github.com/induzo/*"
```

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

- database / pginit

### Depends on contextslogger

- http / handlerwrap

## Contribution

```bash
pre-commit install -t commit-msg -t pre-commit -t pre-push
```
