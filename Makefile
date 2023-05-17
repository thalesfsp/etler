###
# Params.
###

PROJECT_FULL_NAME := etler

HAS_GODOC := $(shell command -v godoc;)
HAS_GOLANGCI := $(shell command -v golangci-lint;)
HAS_DOCKER_COMPOSE := $(shell command -v docker-compose;)

default: ci

###
# Entries.
###

ci: lint test coverage

coverage:
	@go tool cover -func=coverage.out && echo "Coverage OK"

deps:
	@go install golang.org/x/tools/cmd/godoc@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.51.2

doc:
ifndef HAS_GODOC
	@echo "Could not find godoc, installing it and any other missing tool(s)"
	@make deps
endif
	@echo "Open localhost:6060/pkg/github.com/thalesfsp/$(PROJECT_FULL_NAME)/ in your browser\n"
	@godoc -http :6060

lint:
ifndef HAS_GOLANGCI
	@echo "Could not find golangci-list, installing it and any other missing tool(s)"
	@make deps
endif
	@golangci-lint run -v -c .golangci.yml && echo "Lint OK"

test:
	@go test -timeout 30s -short -v -race -cover -coverprofile=coverage.out ./... && echo "Test OK"

.PHONY: ci \
	coverage \
	deps \
	doc \
	@echo "Open localhost \
	@godoc -http  \
	lint \
	test