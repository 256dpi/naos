all: fmt vet lint

fmt:
	go fmt .
	go fmt ./cmd/nadm

vet:
	go vet .
	go vet ./cmd/nadm

lint:
	golint .
	golint ./cmd/nadm

copy:
	gp run "cp ./bin/nadm /usr/local/bin" -r
