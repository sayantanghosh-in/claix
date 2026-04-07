APP_NAME := claix
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build run clean lint test install

build:
	go build -ldflags "-s -w -X github.com/sayantanghosh-in/claix/cmd/claix.version=$(VERSION)" -o bin/$(APP_NAME) .

run: build
	./bin/$(APP_NAME)

clean:
	rm -rf bin/ dist/

lint:
	golangci-lint run ./...

test:
	go test ./... -v -race

install: build
	cp bin/$(APP_NAME) $(GOPATH)/bin/$(APP_NAME)
