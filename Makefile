.PHONY: run-tests clean

.DEFAULT: run-tests

run-tests:
	go test -test.v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

clean:
	test -f coverage.out && rm -f coverage.out || exit 0
