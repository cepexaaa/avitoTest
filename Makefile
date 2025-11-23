.PHONY: generate build test run

generate:
	oapi-codegen -config oapi-codegen.yaml openapi.yml

build:
	go build -o bin/server cmd/server/main.go

test:
	go test ./... -v

run: build
	./bin/server

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

clean:
	docker-compose down -v --remove-orphans

lint:
	golangci-lint run

migrate-up:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "user=postgres password=password dbname=review_service sslmode=disable host=localhost port=5432" up

migrate-down:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "user=postgres password=password dbname=review_service sslmode=disable host=localhost port=5432" down

migrate-status:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "user=postgres password=password dbname=review_service sslmode=disable host=localhost port=5432" status

load-test:
	wrk -t4 -c100 -d30s http://localhost:8080/health

start: docker-up wait-for-db migrate-up

stop: docker-down

wait-for-db:
	@until docker-compose exec -T postgres pg_isready -U postgres; do \
		echo "Waiting for database..."; \
		sleep 2; \
	done
	@echo "Database is ready!"