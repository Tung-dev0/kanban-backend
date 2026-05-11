.PHONY: tidy run build clean

tidy:
	go mod tidy

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

clean:
	rm -rf bin
