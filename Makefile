default: run

dev-dependencies:
ifeq (, $(shell which wtc 2>/dev/null))
	@echo "missing dependency: 'go get -u github.com/rafaelsq/wtc" && false
endif

run: dev-dependencies
	@wtc
sfu: dev-dependencies
	@wtc sfu

p2p: dev-dependencies
	@wtc p2p

build-p2p:
	docker build -t rodrigobrito/p2p -f docker/p2p.Dockerfile .
