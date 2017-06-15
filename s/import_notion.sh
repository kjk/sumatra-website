#!/bin/bash
set -u -e -o pipefail

go run cmd/import_notion_docs/*.go
