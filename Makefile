GITCOMMIT?=$(shell git describe --dirty --always)


BUILD_OPTS=

GOOS:=$(shell go env GOOS)
GOARCH:=$(shell go env GOARCH)

default:
	$(call cross_target,${GOOS},${GOARCH})
	mv bin/dns-forwarder-${GOOS}_${GOARCH} bin/dns-forwarder

define cross_target
	env GOOS=$(1) GOARCH=$(2) go build -tags=gc_opt -tags=poll_opt  -ldflags="-s -w -X dnsforwarder.GitCommit=$(GITCOMMIT)" -o bin/dns-forwarder-$(1)_$(2) github.com/jdamick/dns-forwarder/bin
endef
cross:
	$(call cross_target,windows,386)
	$(call cross_target,windows,amd64)
	$(call cross_target,windows,arm64)
	$(call cross_target,linux,386)
	$(call cross_target,linux,amd64)
	$(call cross_target,linux,arm64)
	$(call cross_target,darwin,amd64)
	$(call cross_target,darwin,arm64)

test:
	go test -v -count=1 ./...

coverage:
	go test -coverprofile=cover.out -covermode=atomic -race ./...; [ -f cover.out ] && cat cover.out >> coverage.txt
