.PHONY: all build generate serve clean test

all: generate build

# Run static site generator
generate:
	go run generator.go

# Build the main binary
build:
	go build -o awen main.go

# Start the web server and host static site + dynamic APIs
serve:
	go run main.go

# Clean compiled assets
clean:
	rm -rf dist awen
