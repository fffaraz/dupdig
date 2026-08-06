FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -tags netgo -ldflags="-s -w -extldflags '-static' -X main.version=${VERSION}" -trimpath -o /dupdig .

FROM scratch
COPY --from=builder /dupdig /dupdig
WORKDIR /data
ENTRYPOINT ["/dupdig"]
