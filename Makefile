APP := $(basename $(git remote get-url origin))
REGISTRY := quay.io/vladklim
VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null)-$(shell git rev-parse --short HEAD)
TARGETOS ?= linux
TARGETARCH ?= arm64

.PHONY: format lint get test build linux macos macos-arm windows image push buildx clean clean-image

format:
	gofmt -s -w ./

lint:
	golint ./...

get:
	go get ./...

test:
	go test -v ./...

build: format
	GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -v -o kbot -ldflags "-X=github.com/ARmrCode/kbot/cmd.appVersion=${VERSION}"

linux:
	${MAKE} TARGETOS=linux TARGETARCH=amd64 build

macos:
	${MAKE} TARGETOS=darwin TARGETARCH=amd64 build

macos-arm:
	${MAKE} TARGETOS=darwin TARGETARCH=arm64 build

windows:
	${MAKE} TARGETOS=windows TARGETARCH=amd64 build

image:
	docker build \
		--build-arg TARGETOS=${TARGETOS} \
		--build-arg TARGETARCH=${TARGETARCH} \
		. -t ${REGISTRY}/${APP}:${VERSION}-${TARGETARCH}

push:
	docker push ${REGISTRY}/${APP}:${VERSION}-${TARGETARCH}

buildx:
	docker buildx build \
		--platform linux/amd64,linux/arm64,windows/amd64 \
		--push \
		--tag ${REGISTRY}/${APP}:${VERSION} \
		.

clean:
	rm -rf kbot

clean-image:
	-docker rmi ${REGISTRY}/${APP}:${VERSION}-${TARGETARCH}
