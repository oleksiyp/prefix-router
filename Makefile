TAG?=latest

build:
	CGO_ENABLED=0 GOOS=linux go build  \
		-ldflags "-s -w \
		-a -installsuffix cgo -o ./bin/prefix-router ./cmd/prefix-router/*
	docker build -t oleksiyp/prefix-router:$(TAG) . -f Dockerfile

fmt:
	gofmt -l -s -w ./
	goimports -l -w ./

test-fmt:
	gofmt -l -s ./ | grep ".*\.go"; if [ "$$?" = "0" ]; then exit 1; fi
	goimports -l ./ | grep ".*\.go"; if [ "$$?" = "0" ]; then exit 1; fi

codegen:
	./hack/update-codegen.sh

test: test-fmt
	go test ./...
