APP := $(shell basename $(shell git remote get-url origin))
REGISTRY := ghcr.io/armrcode
VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)-$(shell git rev-parse --short HEAD)
TARGETOS ?= linux
TARGETARCH ?= amd64

.PHONY: format lint get test build linux macos macos-arm windows image push clean

format:
	gofmt -s -w ./

lint:
	golint ./...

get:
	go mod tidy

test:
	go test -v ./...

build: format get
	GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -v \
		-o kbot \
		-ldflags "-X=github.com/ARmrCode/kbot/cmd.appVersion=${VERSION}"

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
		. -t ${REGISTRY}/${APP}:${VERSION}-${TARGETOS}-${TARGETARCH}

push:
	docker push ${REGISTRY}/${APP}:${VERSION}-${TARGETOS}-${TARGETARCH}

clean:
	rm -rf kbot kbot.exe
	-docker rmi ${REGISTRY}/${APP}:${VERSION}-${TARGETOS}-${TARGETARCH}

	