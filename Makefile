all: fmt vet lint

fmt:
	go fmt .
	go fmt ./mqtt
	go fmt ./ble
	go fmt ./xtensa
	go fmt ./cmd/naos
	go fmt ./utils

vet:
	go vet .
	go vet ./mqtt
	go vet ./ble
	go vet ./xtensa
	go vet ./cmd/naos
	go vet ./utils

lint:
	golint .
	golint ./mqtt
	golint ./ble
	golint ./xtensa
	golint ./cmd/naos
	golint ./utils

install:
	gp run "go install github.com/shiftr-io/naos/cmd/naos"
	gp run "cp ./bin/naos /usr/local/bin" -r
