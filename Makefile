APP := $(shell basename -s .git $(shell git remote get-url origin))
REGISTRY := quay.io/armrcode
VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)-$(shell git rev-parse --short HEAD)
TARGETOS ?= linux
TARGETARCH ?= arm64

.PHONY: format lint get test build linux macos macos-arm windows image push clean clean-image

format:
	gofmt -s -w ./

lint:
	golint ./...

get:
	go mod tidy

test:
	go test -v ./...

build: format
ifeq (${TARGETOS},windows)
	GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -v -o kbot.exe -ldflags "-X=github.com/ARmrCode/kbot/cmd.appVersion=${VERSION}"
else
	GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -v -o kbot -ldflags "-X=github.com/ARmrCode/kbot/cmd.appVersion=${VERSION}"
endif

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

clean:
	rm -rf kbot kbot.exe
	-docker rmi ${REGISTRY}/${APP}:${VERSION}-${TARGETARCH}

clean-image:
	-docker rmi ${REGISTRY}/${APP}:${VERSION}-${TARGETARCH}
	