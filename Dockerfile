FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.23.3-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH

RUN apk add --no-cache make git bash

WORKDIR /build

COPY go.mod go.sum /build/
RUN go mod download
RUN go mod verify

COPY . /build/
RUN make build-binary

FROM --platform=$TARGETPLATFORM docker.io/library/busybox
LABEL maintainer="Robert Jacob <xperimental@solidproject.de>"

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /build/logging-roundtrip /bin/logging-roundtrip

USER nobody
EXPOSE 9205

ENTRYPOINT ["/bin/logging-roundtrip"]
