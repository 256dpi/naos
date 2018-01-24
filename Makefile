all: fmt vet lint

fmt:
	go fmt .
	go fmt ./mqtt
	go fmt ./ble
	go fmt ./tree
	go fmt ./cmd/naos
	go fmt ./utils

vet:
	go vet .
	go vet ./mqtt
	go vet ./ble
	go vet ./tree
	go vet ./cmd/naos
	go vet ./utils

lint:
	golint .
	golint ./mqtt
	golint ./ble
	golint ./tree
	golint ./cmd/naos
	golint ./utils

install:
	go install github.com/shiftr-io/naos/cmd/naos
