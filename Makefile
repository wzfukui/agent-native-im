.PHONY: run build test clean

APP_NAME := agent-native-im
BUILD_DIR := bin

run:
	go run ./cmd/server

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/server

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR) data/*.db
