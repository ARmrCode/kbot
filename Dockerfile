FROM golang:1.19 AS builder

WORKDIR /app

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64

ENV TARGETOS=${TARGETOS}
ENV TARGETARCH=${TARGETARCH}

RUN make build

FROM alpine:latest AS certs
RUN apk --no-cache add ca-certificates

FROM scratch

WORKDIR /

COPY --from=builder /app/kbot /
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/kbot"]