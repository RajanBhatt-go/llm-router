.PHONY: build run serve health test clean

BINARY=llm-router

build:
	go build -o $(BINARY) .

run:
	go run . run "$(PROMPT)"

serve:
	go run . serve --port 8080

health:
	go run . health

config-init:
	go run . config init

test:
	go test ./...

clean:
	rm -f $(BINARY)

docker-up:
	docker compose -f deploy/docker-compose.yml up -d

docker-down:
	docker compose -f deploy/docker-compose.yml down