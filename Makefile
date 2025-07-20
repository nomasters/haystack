.PHONEY: coverage test inspect install sec-scan

test:
	go test -v ./...

coverage:
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...

inspect: coverage
	go tool cover -html=coverage.txt

sec-scan:
	@echo "Installing gosec if not present..."
	@which gosec > /dev/null 2>&1 || go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	@echo "Running security scan with gosec..."
	gosec -fmt=json -out=gosec-report.json -stdout -verbose=text ./...
	@echo "Security scan complete. Report saved to gosec-report.json"

update-deps:
	go get -u && go mod tidy

install:
	go install github.com/nomasters/haystack/cmd/haystack