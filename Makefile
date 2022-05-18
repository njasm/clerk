.PHONY: run-tests clean vet lint

.DEFAULT: test-all

test-all: vet lint run-tests clean

vet:
	@echo "=> Running vet..."
	go vet ./...

lint:
	@echo "=> Running staticcheck..."
	go install honnef.co/go/tools/cmd/staticcheck@latest
	@staticcheck -version
	@staticcheck ./...

run-tests:
	@echo "=> Running go test..."
	go test -test.v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

clean:
	@test -f coverage.out && rm -f coverage.out || exit 0