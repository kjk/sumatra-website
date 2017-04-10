#!/bin/bash
set -u -e -o pipefail

go tool vet -printfuncs=httpErrorf:1,panicif:1,Noticef,Errorf .
GOOS=linux GOARCH=amd64 go build -o sumatra_website_linux
