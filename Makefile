VERSION=$(shell git describe --tags --abbrev=0 2>/dev/null)-$(shell git rev-parse --short HEAD)

format:
	gofmt -s -w ./

build: format
	GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -v -o kbot -ldflags "-X=github.com/ARmrCode/kbot/cmd.appVersion=${VERSION}"
