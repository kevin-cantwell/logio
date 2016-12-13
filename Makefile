tag := $(shell git name-rev --tags --always --name-only HEAD)
ifeq ($(tag),undefined)
tag := $(shell git rev-parse --short HEAD)
endif

all: go docker

go:
	GOOS=linux GOARCH=amd64 go build -o bin/logio-server cmd/logio-server/main.go

docker:
	docker build -t kevincantwell/logio:$(tag) .
	docker tag kevincantwell/logio:$(tag) kevin-cantwell/logio:latest