#!/bin/bash
set -u -e -o pipefail

go tool vet *.go
GOOS=linux GOARCH=amd64 go build -o sumatra_website_linux

docker build --tag sumatra-website:latest .

rm sumatra_website_linux
