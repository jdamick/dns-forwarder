

default:
	go build -tags=gc_opt -tags=poll_opt -o bin/dns-forwarder github.com/jdamick/dns-forwarder/bin
#	go build -tags=gc_opt -tags=poll_opt -o dns-forwarder `go list ./...`

test:
	go test -v ./...
