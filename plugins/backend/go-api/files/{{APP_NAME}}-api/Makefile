.PHONY: dev build run migrate

dev:
	go run .

build:
	go build -o bin/api-server .

run: build
	./bin/api-server

migrate:
	go run . -migrate
