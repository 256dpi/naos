all: fmt vet lint

fmt:
	go fmt .
	go fmt ./cmd/naos

vet:
	go vet .
	go vet ./cmd/naos

lint:
	golint .
	golint ./cmd/naos

install:
	gp run "go install github.com/shiftr-io/naos/cmd/naos"
	gp run "cp ./bin/naos /usr/local/bin" -r
