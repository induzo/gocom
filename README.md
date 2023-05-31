# induzo/gocom

[![security scanner](https://github.com/induzo/gocom/actions/workflows/sec-scanner.yml/badge.svg)](https://github.com/induzo/gocom/actions/workflows/sec-scanner.yml) [![linter](https://github.com/induzo/gocom/actions/workflows/linter.yml/badge.svg)](https://github.com/induzo/gocom/actions/workflows/linter.yml) [![tests](https://github.com/induzo/gocom/actions/workflows/tests.yml/badge.svg)](https://github.com/induzo/gocom/actions/workflows/tests.yml) [![conv commits checker](https://github.com/induzo/gocom/actions/workflows/conv-commits-checker.yml/badge.svg)](https://github.com/induzo/gocom/actions/workflows/conv-commits-checker.yml) [![codecov](https://codecov.io/gh/induzo/gocom/branch/main/graph/badge.svg?token=UBWDRLOYDU)](https://codecov.io/gh/induzo/gocom)

common golang packages

## Current modules

| module                                     | benchmarks                                                    | latest version | report                                                                                                                                                                       | docs                                                                                                                                                        |
| ------------------------------------------ | ------------------------------------------------------------- | -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [contextslogger](contextslogger)           | [benches](https://induzo.github.io/gocom/contextslogger)      | 1.0.0          | [![Go Report Card](https://goreportcard.com/badge/github.com/induzo/gocom/contextslogger)](https://goreportcard.com/report/github.com/induzo/gocom/contextslogger)           | [![Go Reference](https://pkg.go.dev/badge/github.com/induzo/gocom/contextslogger.svg)](https://pkg.go.dev/github.com/induzo/gocom/contextslogger)           |
| [database/pginit](database/pginit)         | [benches](https://induzo.github.io/gocom/database/pginit)     | 1.1.4          | [![Go Report Card](https://goreportcard.com/badge/github.com/induzo/gocom/database/pginit)](https://goreportcard.com/report/github.com/induzo/gocom/database/pginit)         | [![Go Reference](https://pkg.go.dev/badge/github.com/induzo/gocom/database/pginit.svg)](https://pkg.go.dev/github.com/induzo/gocom/database/pginit)         |
| [database/pgtest](database/pgtest)         | [benches](https://induzo.github.io/gocom/database/pgtest)     | 1.0.2          | [![Go Report Card](https://goreportcard.com/badge/github.com/induzo/gocom/database/pgtest)](https://goreportcard.com/report/github.com/induzo/gocom/database/pgtest)         | [![Go Reference](https://pkg.go.dev/badge/github.com/induzo/gocom/database/pgtest.svg)](https://pkg.go.dev/github.com/induzo/gocom/database/pgtest)         |
| [database/pgx-slog](database/pgx-slog)     | [benches](https://induzo.github.io/gocom/database/pgx-slog)   | 1.0.0          | [![Go Report Card](https://goreportcard.com/badge/github.com/induzo/gocom/database/pgx-slog)](https://goreportcard.com/report/github.com/induzo/gocom/database/pgx-slog)     | [![Go Reference](https://pkg.go.dev/badge/github.com/induzo/gocom/database/pgx-slog.svg)](https://pkg.go.dev/github.com/induzo/gocom/database/pgx-slog)     |
| [database/redisinit](database/redisinit)   | [benches](https://induzo.github.io/gocom/database/redisinit)  | 1.0.1          | [![Go Report Card](https://goreportcard.com/badge/github.com/induzo/gocom/database/redisinit)](https://goreportcard.com/report/github.com/induzo/gocom/database/redisinit)   | [![Go Reference](https://pkg.go.dev/badge/github.com/induzo/gocom/database/redisinit.svg)](https://pkg.go.dev/github.com/induzo/gocom/database/redisinit)   |
| [database/redistest](database/redistest)   | [benches](https://induzo.github.io/gocom/database/redistest)  | 1.0.1          | [![Go Report Card](https://goreportcard.com/badge/github.com/induzo/gocom/database/redistest)](https://goreportcard.com/report/github.com/induzo/gocom/database/redistest)   | [![Go Reference](https://pkg.go.dev/badge/github.com/induzo/gocom/database/redistest.svg)](https://pkg.go.dev/github.com/induzo/gocom/database/redistest)   |
| [http/handlerwrap](http/handlerwrap)       | [benches](https://induzo.github.io/gocom/http/handlerwrap)    | 0.1.0          | [![Go Report Card](https://goreportcard.com/badge/github.com/induzo/gocom/http/handlerwrap)](https://goreportcard.com/report/github.com/induzo/gocom/http/handlerwrap)       | [![Go Reference](https://pkg.go.dev/badge/github.com/induzo/gocom/http/handlerwrap.svg)](https://pkg.go.dev/github.com/induzo/gocom/http/handlerwrap)       |
| [http/health](http/health)                 | [benches](https://induzo.github.io/gocom/http/health)         | 1.0.1          | [![Go Report Card](https://goreportcard.com/badge/github.com/induzo/gocom/http/health)](https://goreportcard.com/report/github.com/induzo/gocom/http/health)                 | [![Go Reference](https://pkg.go.dev/badge/github.com/induzo/gocom/http/health.svg)](https://pkg.go.dev/github.com/induzo/gocom/http/health)                 |
| [monitoring/otelinit](monitoring/otelinit) | [benches](https://induzo.github.io/gocom/monitoring/otelinit) | 1.1.0          | [![Go Report Card](https://goreportcard.com/badge/github.com/induzo/gocom/monitoring/otelinit)](https://goreportcard.com/report/github.com/induzo/gocom/monitoring/otelinit) | [![Go Reference](https://pkg.go.dev/badge/github.com/induzo/gocom/monitoring/otelinit.svg)](https://pkg.go.dev/github.com/induzo/gocom/monitoring/otelinit) |
| [shutdown](shutdown)                       | [benches](https://induzo.github.io/gocom/shutdown)            | 1.1.0          | [![Go Report Card](https://goreportcard.com/badge/github.com/induzo/gocom/shutdown)](https://goreportcard.com/report/github.com/induzo/gocom/shutdown)                       | [![Go Reference](https://pkg.go.dev/badge/github.com/induzo/gocom/shutdown.svg)](https://pkg.go.dev/github.com/induzo/gocom/shutdown)                       |

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
