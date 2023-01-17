THIS_FILE := $(lastword $(MAKEFILE_LIST))""
GOFLAGS=-mod=mod

#help: @ List available tasks
help:
	@clear
	@echo "Usage: make COMMAND"
	@echo "Commands :"
	@grep -E '[a-zA-Z\.\-]+:.*?@ .*$$' $(MAKEFILE_LIST)| tr -d '#' | awk 'BEGIN {FS = ":.*?@ "}; {printf "\033[32m%-19s\033[0m - %s\n", $$1, $$2}'

#test: @ Run tests
test:
	@go generate
	@export GOFLAGS=$(GOFLAGS); export TZ="UTC"; go test -v ./...

#update: @ Update dependencies to latest versions
update:
	@export GOFLAGS=$(GOFLAGS); go get -u ./...; go mod tidy

#jwt-create: @ Create JWT file
jwt-create:
	./jwt-create.sh
