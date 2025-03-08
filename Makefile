.PHONY: default
default: test

.PHONY: install
install:
	go install mvdan.cc/gofumpt@latest
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.64.6

.PHONY: lint
lint:
	go list -f {{.Dir}} ./... | xargs gofumpt -w
	golangci-lint version
	golangci-lint run ./...

.PHONY: test
test: lint
	go mod tidy
	go test -bench=. --race ./...