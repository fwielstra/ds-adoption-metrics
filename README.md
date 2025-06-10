# CRNT Adoption Metrics Generator

A tool that queries the Gitlab advanced search API at a fixed interval with a set of premade queries to track adoption metrics of the CRNT Design System.

There should be two sets of queries, one for the 'existing' components that CRNT components will replace, and one for CRNT components. Ideally we'll see a 1:1 conversion ratio between the two.

## TODO

- [x] Project setup
- sqlite database OR logfiles
- connect with Gitlab API
  - https://docs.gitlab.com/api/search/
- define queries
  - just hardcode, it's not hard
  - unique name & query
- extract data; number of results, output clickable links
- generate charts?

## Getting started

- install [Go](https://go.dev/doc/install)
- install [Just](https://github.com/casey/just)
  - `./scripts/install-just.sh --to $(go env GOPATH)/bin`
- install [golangci-lint](https://github.com/golangci/golangci-lint)
  - `./scripts/install-golangci-lint.sh -b $(go env GOPATH)/bin v2.1.5`
- install [watchexec](https://github.com/watchexec/watchexec) via [apt.cli.rs](https://apt.cli.rs/) repository:
  - `./scripts/install-watchexec.sh`

## Running

    just run

## Running in watch mode

    just watch

## Building

    just build

## Testing

    just test
