.PHONY: build test run clean

# Build the application
build:
	@echo "Building compass..."
	@mkdir -p bin
	@go build -o bin/compass ./cmd/compass

# Run tests
test: build
	@echo "Running tests..."
	@go test -v ./...

# Run the application
run: build
	@echo "Running compass..."
	@./bin/compass --dev

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin
