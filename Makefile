.DEFAULT_GOAL := help
.PHONY: help web server quote dev up build test clean

help:
	@printf '%s\n' 'make dev up        Start quote, web service, and frontend; Ctrl+C stops all' \
		'make web dev       Start the frontend only' \
		'make web build     Build the frontend' \
		'make web test      Run frontend browser tests' \
		'make server dev    Start the backend with confg/dev.json' \
		'make server build  Build the backend to bin/app' \
		'make server test   Run backend tests with coverage in gen/coverage.out' \
		'make quote dev     Start the mock quote service with config/dev.json' \
		'make quote build   Build the mock quote service to test/mock/commissionquote/bin/quote' \
		'make clean         Stop Vite and remove dependencies and generated files'

# Make treats the component and the following action as separate targets.
web server quote:
	@:

ifneq ($(word 2,$(sort $(filter web server quote,$(MAKECMDGOALS)))),)
$(error Choose only one component: web, server, or quote)
endif

ifneq ($(filter up,$(MAKECMDGOALS)),)
ifneq ($(sort $(MAKECMDGOALS)),dev up)
$(error Use make dev up to start all components)
endif
dev:
	@:

up: SHELL := /bin/bash
up:
	@set -m; \
	pids=""; \
	trap 'for pid in $$pids; do kill -TERM -- -$$pid 2>/dev/null || true; done; wait' EXIT; \
	trap 'exit 0' INT TERM; \
	make quote dev & pids="$$pids $$!"; \
	make server dev & pids="$$pids $$!"; \
	make web dev & pids="$$pids $$!"; \
	wait
else ifneq ($(filter quote,$(MAKECMDGOALS)),)
dev:
	cd test/mock/commissionquote && go run ./cmd --config config/dev.json

build:
	mkdir -p test/mock/commissionquote/bin
	cd test/mock/commissionquote && go build -o bin/quote ./cmd

test:
	$(error make quote supports dev and build only)
else ifneq ($(filter server,$(MAKECMDGOALS)),)
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
	rm -rf test/mock/commissionquote/bin test/mock/commissionquote/gen
	rm -f app coverage.out coverage.html
