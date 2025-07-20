.PHONEY: coverage test inspect install sec-scan

test:
	go test -v ./...

coverage:
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...

inspect: coverage
	go tool cover -html=coverage.txt

sec-scan:
	gosec -fmt=json -out=gosec-report.json -stdout -verbose=text ./...

update-deps:
	go get -u && go mod tidy

install:
	go install github.com/nomasters/haystack/cmd/haystack