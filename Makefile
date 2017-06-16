all: fmt vet lint

fmt:
	go fmt .
	go fmt ./fleet
	go fmt ./cmd/naos

vet:
	go vet .
	go vet ./fleet
	go vet ./cmd/naos

lint:
	golint .
	golint ./fleet
	golint ./cmd/naos

install:
	gp run "go install github.com/shiftr-io/naos/cmd/naos"
	gp run "cp ./bin/naos /usr/local/bin" -r
