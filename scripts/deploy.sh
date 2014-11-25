#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

GOOS=linux GOARCH=amd64 go build -o sumatra_website_linux
fab deploy
rm sumatra_website_linux
