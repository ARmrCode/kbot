FROM quay.io/projectquay/golang:1.23 AS builder

WORKDIR /app

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN make build TARGETOS=${TARGETOS} TARGETARCH=${TARGETARCH}

FROM alpine:latest AS certs
RUN apk --no-cache add ca-certificates

FROM scratch

WORKDIR /

COPY --from=builder /app/kbot /kbot
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/kbot", "start"]