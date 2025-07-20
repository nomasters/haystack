.PHONEY: coverage test inspect install sec-scan lint fmt check

test:
	go test -v ./...

coverage:
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...

inspect: coverage
	go tool cover -html=coverage.txt

sec-scan:
	gosec -fmt=json -out=gosec-report.json -stdout -verbose=text ./...

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

check: fmt lint test
	@echo "All checks passed!"

update-deps:
	go get -u && go mod tidy

install:
	go install github.com/nomasters/haystack/cmd/haystack