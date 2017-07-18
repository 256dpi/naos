all: fmt vet lint

fmt:
	go fmt .
	go fmt ./mqtt
	go fmt ./ble
	go fmt ./cmd/naos

vet:
	go vet .
	go vet ./mqtt
	go vet ./ble
	go vet ./cmd/naos

lint:
	golint .
	golint ./mqtt
	golint ./ble
	golint ./cmd/naos

install:
	gp run "go install github.com/shiftr-io/naos/cmd/naos"
	gp run "cp ./bin/naos /usr/local/bin" -r
