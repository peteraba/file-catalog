.PHONY: install
install:
	go install mvdan.cc/gofumpt@latest
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.60.3

.PHONY: lint
lint:
	go list -f {{.Dir}} ./... | xargs gofumpt -w
	golangci-lint run ./...

.PHONY: test
test: lint
	go mod tidy
	go test -bench=. --race ./...