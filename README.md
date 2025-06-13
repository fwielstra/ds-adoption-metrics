# CRNT Adoption Metrics Generator

A tool that queries the Gitlab advanced search API at a fixed interval with a set of premade queries to track adoption metrics of the CRNT Design System.

There should be two sets of queries, one for the 'existing' components that CRNT components will replace, and one for CRNT components. Ideally we'll see a 1:1 conversion ratio between the two.

## Overview

This tool creates an sqlite database in `data/adoption.db`, then queries [the Gitlab REST API](https://docs.gitlab.com/api/rest/) at https://gitlab.essent.nl/api/v4.

It first fetches all projects to get a readable name, then runs a set of search queries to [find specific fragments of code](https://docs.gitlab.com/api/search/#scope-blobs).

The main logic is contained in [`main.go`](./main.go); database specific logic is in the `sqlite` folder, and a gitlab service layer is contained in the `glclient` folder.

## TODO

- [x] Project setup
- sqlite database OR logfiles
- [x] connect with Gitlab API
  - https://docs.gitlab.com/api/search/
- [x] define queries
  - just hardcode, it's not hard
  - unique name & query
- [x] extract data; number of results, output clickable links?
- lookup project names by ID
- [x] output basic stats on commandline:
  - total results
  - results per project
- [x] generate charts?
- generate "adoption" output; pair of queries (old & new), compare 'oldest' results with 'newest' and calculate "conversion percentage".
- Maintain database version, run migrations or just reset database and do a clean fetch
- commandline commands for e.g. dropping projects cache
  - use spf13/cobra
  - command 'serve'
  - command 'update' to fetch latest data (is that the right name?)
  - optional command 'reset' to reset data
- Properly structure application:
  - business and domain logic in top level defining client interfaces
  - [x] gitlab service layer wrapping the gitlab library (domain <-> gitlab)
  - [x] database layer wrapping database access / storage (domain <-> database)
- ?? chart generating layer (domain -> visualisation)
- should we do something with context? e.g. timeout and cancellation support
- [-] switch to using [the graphql api](https://docs.gitlab.com/api/graphql/) since we throw away a lot of data from the REST API.
  - see [graphql-explorer](https://gitlab.essent.nl/-/graphql-explorer)
  - We may get away with fetching all data (like projects) in one go then.
  - graphql server does not seem to support blob search properly / like we want it.

## Getting started

- install [Go](https://go.dev/doc/install)
- install [Just](https://github.com/casey/just)
  - `./scripts/install-just.sh --to $(go env GOPATH)/bin`
- install [golangci-lint](https://github.com/golangci/golangci-lint)
  - `./scripts/install-golangci-lint.sh -b $(go env GOPATH)/bin v2.1.5`
- install [watchexec](https://github.com/watchexec/watchexec) via [apt.cli.rs](https://apt.cli.rs/) repository:
  - `./scripts/install-watchexec.sh`
- (optional for debugging) install sqlite

## Running

### Reading stored data

The default command pulls data from the local database in `data/adoption.db` and outputs it as a table. To invoke it as developer, run:

    just run

### Updating data

Generate a Gitlab access key with the `read_api` permissions from [your settings](https://gitlab.essent.nl/-/user_settings/personal_access_tokens).

Either set this as an environment variable called `PRIVATE_TOKEN`, or pass it when invoking the `run` command:

    PRIVATE_TOKEN=abcdefghijklmnop just run -update -verbose

## Running in watch mode

    just watch

## Building

    just build

## Testing

    just test
