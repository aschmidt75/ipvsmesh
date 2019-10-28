VERSION := `cat VERSION`
SOURCES ?= $(shell find . -name "*.go" -type f)
BINARY_NAME = ipvsmesh
PKG = github.com/aschmidt75/ipvsmesh

all: clean vet lint gen cover build

.PHONY: build
build:
	CGO_ENABLED=0 go build -i -v -o release/${BINARY_NAME} -ldflags="-X main.version=${VERSION}" cmd/*.go

vet:
	go vet cmd/*.go

lint:
	@for file in ${SOURCES} ;  do \
		golint $$file ; \
	done

.PHONY: test
test:
	go test ./...

.PHONY: cover
cover:
	go test -coverprofile=cover.out ./...
	go tool cover -func=cover.out

.PHONY: cover-html
cover-html: cover
	go tool cover -html=cover.out

.PHONY: gen
gen:
	rm -f pkg/mock/*
	go generate ./...

.PHONY: clean
clean:
	rm -rf release/*
	rm -f cover.out
	#go clean -testcache
