FROM golang:1.26-alpine AS builder
ARG VERSION=dev
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -trimpath -o /dupdig .

FROM scratch
COPY --from=builder /dupdig /dupdig
ENTRYPOINT ["/dupdig"]
