.PHONY: build test run clean

# Build the application
build:
	@echo "Building todo..."
	@mkdir -p bin
	@go build -o bin/todo ./cmd/todo

# Run tests
test: build
	@echo "Running tests..."
	@go test -v ./...

# Run the application
run: build
	@echo "Running todo..."
	@./bin/todo --dev

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin
