test:
	go test $(shell go list ./... | grep -v /examples/ ) -covermode=count

test-race:
	go test -race $(shell go list ./... | grep -v /examples/ )

coverage:
	GO111MODULE=off go test ./... -cover -covermode=count -coverprofile=cover.out; GO111MODULE=off go tool cover -func cover.out;

coverage-html:
	GO111MODULE=off go test ./... -cover -covermode=count -coverprofile=cover.out; GO111MODULE=off go tool cover -html=cover.out;

benchmarks:
	cd benchmarks && go test -bench . && cd ../

lint: 
	golint -set_exit_status $(shell (go list ./... | grep -v /vendor/))

.PHONY: test test-race coverage coverage-html lint benchmarks mocks
