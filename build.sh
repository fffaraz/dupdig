#!/bin/sh
set -e

rm -f dupdig
CGO_ENABLED=0 go build -ldflags='-s -w' -trimpath -o dupdig .
