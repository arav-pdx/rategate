SHELL := /bin/bash

.PHONY: all build test certs run docker-up docker-down bench clean

all: build

certs:
	@mkdir -p certs
	@bash scripts/generate_keys.sh

build: certs
	go build -o bin/rategate ./cmd/gateway

test:
	go test -v -race ./...

docker-up: certs
	docker compose -f deploy/docker-compose.yml up --build -d

docker-down:
	docker compose -f deploy/docker-compose.yml down -v

bench:
	@test -n "$(TOKEN)" || (echo "Usage: make bench TOKEN=<jwt_token>" && exit 1)
	k6 run -e TOKEN=$(TOKEN) bench/load_test.js

clean:
	rm -rf bin certs
