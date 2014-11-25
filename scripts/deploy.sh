#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

rm -rf $TMPDIR/godep
GOOS=linux GOARCH=amd64 godep go build -o sumatra_website_linux
fab deploy
rm sumatra_website_linux
