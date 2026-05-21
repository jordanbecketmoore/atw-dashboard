IMAGE ?= atw-dashboard:latest
CONFIG ?= $(PWD)/config.yaml

.PHONY: build run-local tidy fmt vet test docker run clean

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/atw-dashboard ./cmd/server

run-local: build
	./bin/atw-dashboard -config $(CONFIG)

tidy:
	go mod tidy

fmt:
	gofmt -s -w .

vet:
	go vet ./...

test:
	go test ./...

docker:
	docker build -t $(IMAGE) .

run: docker
	docker run --rm -p 8080:8080 -v $(CONFIG):/etc/atw-dashboard/config.yaml:ro $(IMAGE)

clean:
	rm -rf bin/
