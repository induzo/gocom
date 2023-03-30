# golang-common

[![security scanner](https://github.com/induzo/gocom/actions/workflows/sec-scanner.yml/badge.svg)](https://github.com/induzo/gocom/actions/workflows/sec-scanner.yml) [![linter](https://github.com/induzo/gocom/actions/workflows/linter.yml/badge.svg)](https://github.com/induzo/gocom/actions/workflows/linter.yml) [![test & coverage](https://github.com/induzo/gocom/actions/workflows/coverage.yml/badge.svg)](https://github.com/induzo/gocom/actions/workflows/coverage.yml) [![conv commits checker](https://github.com/induzo/gocom/actions/workflows/conv-commits-checker.yml/badge.svg)](https://github.com/induzo/gocom/actions/workflows/conv-commits-checker.yml) [![Coverage Status](https://coveralls.io/repos/github/induzo/gocom/badge.svg?t=cPxXZ8)](https://coveralls.io/github/induzo/gocom)

common golang packages

## Current modules

| module                                     | benchmarks  | latest version |
| ------------------------------------------ | ----------- | -------------- |
| [monitoring/otelinit](monitoring/otelinit) | [benches]() | 0.1.0          |

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

## Other modules (TODO)

- [ ] use slog
- [ ] reset git
- [ ] update pgx/v5 sqlc
- [ ] bench
