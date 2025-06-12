#!/bin/bash

# devcontainer post-create commands, installs additional tools

# executing scripts relative to 'self' is complicated, apparently:
# https://stackoverflow.com/questions/6659689/referring-to-a-file-relative-to-executing-script
cd "$(dirname "${BASH_SOURCE[0]}")"

./install-just.sh --to $(go env GOPATH)/bin
./install-golangci-lint.sh -b $(go env GOPATH)/bin v2.1.5

sudo apt update && sudo apt upgrade
sudo apt install sqlite3