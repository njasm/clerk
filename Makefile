.PHONY: run-tests clean vet lint

.DEFAULT: test-all

test-all: vet lint run-tests

vet:
	@echo "=> Running vet..."
	go vet -tests ./...

lint:
	@echo "=> Running staticcheck..."
	go install honnef.co/go/tools/cmd/staticcheck@latest
	@staticcheck -version
	@staticcheck ./...

run-tests:
	@echo "=> Running go test..."
	go test -test.v -race -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -func=coverage.txt

clean:
	@test -f coverage.txt && rm -f coverage.txt || exit 0
