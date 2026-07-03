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

COVERAGE_MIN := 90

ci: lint test coverage

coverage:
	@go tool cover -func=coverage.out | tail -1 | awk -v min=$(COVERAGE_MIN) '{gsub("%","",$$3); if ($$3+0 < min) { printf "Coverage %.1f%% is BELOW the minimum %d%%\n", $$3, min; exit 1 } printf "Coverage OK: %.1f%% (minimum %d%%)\n", $$3, min }'

deps:
	@go install golang.org/x/tools/cmd/godoc@latest
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2

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