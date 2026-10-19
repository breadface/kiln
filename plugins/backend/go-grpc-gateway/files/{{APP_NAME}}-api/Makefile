.PHONY: dev build run migrate proto-update proto-lint proto-format proto-gen

dev:
	go run .

build:
	go build -o bin/api-server .

run: build
	./bin/api-server

migrate:
	go run . -migrate

# ── Proto / buf ───────────────────────────────────────────────────────────
# buf.yaml, buf.gen.yaml, and proto/ live at the repo root (one level up)
# so another plugin (e.g. a mobile client) can generate its own stubs from
# the same source without depending on this directory.

proto-update:
	cd .. && buf dep update

proto-lint: proto-update
	cd .. && buf lint proto

proto-format:
	cd .. && buf format proto -w

proto-gen: proto-lint proto-format
	cd .. && buf generate
