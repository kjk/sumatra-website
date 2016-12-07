#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

hyper fip detach sumatra-website
hyper stop sumatra-website
hyper rm sumatra-website
hyper run --size=s3 --restart=unless-stopped -d -p 80 -v sumatra-website:/data --name sumatra-website sumatra-website:latest
hyper fip attach 209.177.91.155 sumatra-website
