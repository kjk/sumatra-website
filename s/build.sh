#!/bin/bash
set -u -e -o pipefail

go build -o sumatra_website -ldflags "-X main.sha1ver=`git rev-parse HEAD`"
rm sumatra_website
