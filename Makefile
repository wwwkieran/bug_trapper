BINARY := bug_trapper

.PHONY: build build-gocv build-pi build-pi-arm cross-pi cross-pi-arm \
        run test test-integration clean tidy fmt vet hw-test

build:
	go build -o $(BINARY) .

build-gocv:
	go build -tags gocv -o $(BINARY) .

build-pi:
	go build -tags pi -o $(BINARY) .

cross-pi:
	CGO_ENABLED=1 CC="zig cc -target aarch64-linux-gnu" \
		GOOS=linux GOARCH=arm64 go build -tags pi -o $(BINARY) .

cross-pi-arm:
	CGO_ENABLED=1 CC="zig cc -target arm-linux-gnueabihf" \
		GOOS=linux GOARCH=arm GOARM=7 go build -tags pi -o $(BINARY) .

run: build
	./$(BINARY)

test:
	go test ./...

test-integration:
	go test -tags=integration ./camera/

hw-test: build-pi
	sudo ./$(BINARY) --hw-test all

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)
