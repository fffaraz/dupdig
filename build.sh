#!/bin/sh
set -e

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo dev)

rm -f dupdig
CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -trimpath -o dupdig .
