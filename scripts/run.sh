#!/bin/bash
set -u -e -o pipefail

go build -o sumatra_website
./sumatra_website
rm sumatra_website
