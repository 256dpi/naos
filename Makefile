all: fmt vet lint

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golint .
	golint ./fleet
	golint ./tree
	golint ./cmd/naos
	golint ./utils

test:
	go test ./...

install:
	go install github.com/shiftr-io/naos/cmd/naos
