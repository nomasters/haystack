.PHONEY: coverage test inspect

test:
	go test -v ./...

coverage:
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...

inspect: coverage
	go tool cover -html=coverage.txt

update-deps:
	go get -u && go mod tidy