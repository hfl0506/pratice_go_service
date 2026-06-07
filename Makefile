dev:
	go run cmd/main.go

build:
	go build -o server cmd/main.go

fmt:
	go fmt ./...

test:
	go test ./...
