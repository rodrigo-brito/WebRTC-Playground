default: run

dev-dependencies:
ifeq (, $(shell which wtc 2>/dev/null))
	@echo "missing dependency: 'go get -u github.com/rafaelsq/wtc" && false
endif

run: dev-dependencies
	@wtc
