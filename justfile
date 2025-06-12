set positional-arguments

build: binfolder
  go build -o bin ./...

run *args='': build
  ./bin/crntmetrics $@

watch:
  watchexec --exts go --ignore bin just run

test:
  go test ./...

format:
  gofmt -s -w .

lint:
  golangci-lint run

clean:
  rm -rf bin

# utils
binfolder:
  mkdir -p bin
