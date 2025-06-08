COVER_FILE ?= coverage.out

.PHONY: default
default: help

.PHONY: lint
lint: ## Check the project with lint.
	@go tool golangci-lint run -v --fix

.PHONY: test
test: ## Run unit tests
	@go test ./... -coverprofile=$(COVER_FILE)
	@go tool cover -func=$(COVER_FILE) | grep ^total

.PHONY: test-race
test-race: ## Run unit test and race detector.
	@go test -race ./... -coverprofile=$(COVER_FILE)

.PHONY: cover
cover: $(COVER_FILE) ## Output coverage in human readable form in html
	@go tool cover -html=$(COVER_FILE)
	@rm -f $(COVER_FILE)

.PHONY: mod
mod: ## Manage go mod dependencies, beautify go.mod and go.sum files.
	@go tool go-mod-upgrade
	@go mod tidy

.PHONY: help
help: ## Prints this help message
	@echo "Commands:"
	@grep -F -h '##' $(MAKEFILE_LIST) \
		| grep -F -v fgrep \
		| sort \
		| grep -E '^[a-zA-Z_-]+:.*?## .*$$' \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-30s\033[0m %s\n", $$1, $$2}'
