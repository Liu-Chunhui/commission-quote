.DEFAULT_GOAL := help
.PHONY: help web server dev build test clean

help:
	@printf '%s\n' 'make web dev    Start the frontend only' \
		'make web build  Build the frontend' \
		'make web test   Run frontend browser tests' \
		'make server dev    Start the backend with confg/dev.json' \
		'make server build  Build the backend to bin/app' \
		'make server test   Run backend tests with coverage in gen/coverage.out' \
		'make clean      Stop Vite and remove dependencies and generated files'

# Make treats the component and the following action as separate targets.
web server:
	@:

ifneq ($(filter server,$(MAKECMDGOALS)),)
ifneq ($(filter web,$(MAKECMDGOALS)),)
$(error Choose either web or server for each make invocation)
endif

dev:
	go run ./cmd/app --config confg/dev.json

build:
	mkdir -p bin
	go build -o bin/app ./cmd/app

test:
	mkdir -p gen
	go test ./... -coverpkg=./... -coverprofile=gen/coverage.out
else
dev build test: web/node_modules/.package-lock.json
	npm --prefix web run $@
endif

# Install once, and reinstall when either dependency manifest changes.
web/node_modules/.package-lock.json: web/package.json web/package-lock.json
	npm --prefix web ci

clean:
	@# Match this worktree's Vite executable, including invocations with arguments.
	@for pid in $$(lsof -t -a -d cwd "$(CURDIR)/web" 2>/dev/null); do \
		case "$$(ps -p "$$pid" -o command=)" in \
			*"$(CURDIR)/web/node_modules/.bin/vite"|*"$(CURDIR)/web/node_modules/.bin/vite "*) \
				kill "$$pid" ;; \
		esac; \
	done
	rm -rf bin gen web/node_modules web/dist web/test-results web/playwright-report
	rm -f app coverage.out coverage.html
