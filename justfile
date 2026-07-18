# default recipe: show available commands
default:
    @just --list

# format all Go files
fmt:
    go fmt ./...

# lint with go vet
lint:
    go vet ./...

# run all tests with race detector
test:
    go test -race -count=1 ./...

# build the server binary
build:
    go build -ldflags="-s -w" -o bin/server ./cmd/server

# run the server
run: build
    ./bin/server

# generate an Argon2id password hash for the admin interface
hash:
    go run ./cmd/generate-hash
