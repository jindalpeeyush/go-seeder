.PHONY: all build test clean

APP_NAME=seeder
MAIN_PATH=./cmd/seeder

all: test build

build:
	@echo "Building..."
	go build -o $(APP_NAME) $(MAIN_PATH)

test:
	@echo "Running tests..."
	go test -v -race ./...

lint:
	@echo "Running linter..."
	golangci-lint run

clean:
	@echo "Cleaning up..."
	rm -f $(APP_NAME)
