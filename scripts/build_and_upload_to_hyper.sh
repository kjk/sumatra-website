#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

go tool vet *.go
GOOS=linux GOARCH=amd64 go build -o sumatra_website_linux

docker build --tag kjksf/sumatra-website:latest --tag sumatra-website:latest .

rm sumatra_website_linux

docker push kjksf/sumatra-website:latest
hyper pull kjksf/sumatra-website:latest
