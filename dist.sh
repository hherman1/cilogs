#!/bin/bash

set -e
set -x
rm -rf build/*

GOOS=darwin GOARCH=amd64 go build -o build/cilogs .
zip -j build/darwin-amd64.zip build/cilogs
GOOS=darwin GOARCH=arm64 go build -o build/cilogs .
zip -j build/darwin-arm64.zip build/cilogs
GOOS=linux GOARCH=amd64 go build -o build/cilogs .
zip -j build/linux-amd64.zip build/cilogs
GOOS=windows GOARCH=amd64 go build -o build/cilogs .
zip -j build/windows-amd64.zip build/cilogs