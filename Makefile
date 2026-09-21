.DEFAULT_GOAL := help
.PHONY: help web dev build test clean

help:
	@printf '%s\n' 'make web dev    Start the frontend only' \
		'make web build  Build the frontend' \
		'make web test   Run frontend browser tests' \
		'make clean      Stop Vite and remove frontend dependencies and generated files'

# Make treats "web" and the following action as separate targets.
web:
	@:

dev build test: web/node_modules/.package-lock.json
	npm --prefix web run $@

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
	rm -rf web/node_modules web/dist web/test-results web/playwright-report
