#!/bin/sh
set -e

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo dev)

rm -f dupdig
CGO_ENABLED=0 go build -tags netgo -ldflags="-s -w -extldflags '-static' -X main.version=${VERSION}" -trimpath -o dupdig .
