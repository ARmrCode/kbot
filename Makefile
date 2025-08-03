APP=$(basename $(git remote get-url origin))
REGISTRY=klimik
VERSION=$(shell git describe --tags --abbrev=0 2>/dev/null)-$(shell git rev-parse --short HEAD)
TARGETOS=linux
TARGETARCH=arm64

format:
	gofmt -s -w ./

lint:
	golint

get:
	go get

test:
	go test -v

build: format
	GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -v -o kbot -ldflags "-X=github.com/ARmrCode/kbot/cmd.appVersion=${VERSION}"

image:
	docker build . -t ${REGISTRY}/${APP}:${VERSION}-${TARGETARCH}

push:
	docker push ${REGISTRY}/${APP}:${VERSION}-${TARGETARCH}

clean:
	rm -rf kbot