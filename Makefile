.PHONY: help tidy build run test vet fmt docker-connector docker-internal tilt-up tilt-down

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-22s %s\n", $$1, $$2}'

tidy: ## go mod tidy
	go mod tidy

build: ## go build all
	go build ./...

run: ## run the connector locally (requires env: OAUTH_MS_PROVIDERS_GITHUB_CLIENT_ID/_SECRET, OAUTH_MS_INTERNAL_API_KEY)
	go run ./cmd/server

run-mock: ## run the mock internal service on :8081
	go run ./mocks/internal-service

test: ## run unit tests with race detector
	go test ./... -race -count=1

vet: ## go vet
	go vet ./...

fmt: ## gofmt -s -w
	gofmt -s -w .

docker-connector: ## build the connector image as oauth-ms-connector:dev
	docker build -t oauth-ms-connector:dev .

docker-internal: ## build the mock image as oauth-ms-internal:dev
	docker build -t oauth-ms-internal:dev -f mocks/internal-service/Dockerfile .

tilt-up: ## bring up the local k8s dev loop
	cd deploy/tilt && tilt up

tilt-down: ## tear down the local k8s dev loop
	cd deploy/tilt && tilt down
