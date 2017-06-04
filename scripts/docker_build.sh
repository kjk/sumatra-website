#!/bin/bash
set -u -e -o pipefail

dep ensure
GOOS=linux GOARCH=amd64 go build -o sumatra_website_linux -ldflags "-X main.sha1ver=`git rev-parse HEAD`"
docker build --no-cache --tag sumatrawebsite:latest .
rm sumatra_website_linux
