FROM golang:alpine AS builder
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -ldflags='-s -w' -trimpath -o /dupdig .

FROM scratch
COPY --from=builder /dupdig /dupdig
ENTRYPOINT ["/dupdig"]
