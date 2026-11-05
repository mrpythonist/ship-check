BINARY := shipcheck
VERSION ?= v0.1.0

.PHONY: build test vet fmt clean install

build:
	go build -trimpath -ldflags "-s -w -X github.com/mrpythonist/ship-check/internal/cli.Version=$(VERSION)" -o $(BINARY) ./cmd/shipcheck

test: vet
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l cmd internal pkg

install: build
	cp $(BINARY) /usr/local/bin/$(BINARY)

clean:
	rm -f $(BINARY)
