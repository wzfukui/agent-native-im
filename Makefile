.PHONY: run build test clean web

APP_NAME := agent-native-im
BUILD_DIR := bin

run:
	go run ./cmd/server

build: web
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/server

web:
	cd web && npm install && npm run build

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR) data/*.db web/dist
