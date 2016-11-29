#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

go tool vet *.go
GOOS=linux GOARCH=amd64 go build -o sumatra_website_linux

echo "running: docker build"
docker build --tag sumatra-website:latest .
rm sumatra_website_linux

echo "running: docker save"
docker save sumatra-website:latest | gzip | aws s3 cp - s3://kjkpub/tmp/sumatra.tar.gz
echo "sleeping for 5 secs"
sleep 5
echo "running: hyper load"
hyper load -i $(aws s3 presign s3://kjkpub/tmp/sumatra.tar.gz)
aws s3 rm s3://kjkpub/tmp/sumatra.tar.gz
