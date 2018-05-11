all: fmt vet lint test

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golint ./cmd/naos
	golint ./pkg/naos
	golint ./pkg/fleet
	golint ./pkg/tree
	golint ./pkg/utils

test:
	go test ./...

install:
	go install github.com/shiftr-io/naos/cmd/naos
