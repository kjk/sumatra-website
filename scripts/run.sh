#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

#rm -rf $TMPDIR/godep
go tool vet -printfuncs=httpErrorf:1,panicif:1,Noticef,Errorf .
#godep go build -o blog_app *.go
go build -o sumatrapdfreader
./sumatrapdfreader
rm sumatrapdfreader
