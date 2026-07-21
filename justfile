tag := `git rev-parse --short HEAD`
release_version := `git describe --tags --abbrev=0`

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

build-dev-image:
    docker build -t dockershepherd/replyforge:dev-{{ tag }} .

push-dev-image:
    docker push dockershepherd/replyforge:dev-{{ tag }}

release-dev-image: build-dev-image push-dev-image

# build the release docker image (tagged latest + git version)
build-release-image:
    docker build -t dockershepherd/replyforge:latest -t dockershepherd/replyforge:{{ release_version }} .

# push the release docker image
push-release-image:
    docker push dockershepherd/replyforge:latest
    docker push dockershepherd/replyforge:{{ release_version }}

# build and push the release docker image
release-image: build-release-image push-release-image
