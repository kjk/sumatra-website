#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

#rm -rf $TMPDIR/godep
#godep go build -o sumatrapdfreader
go tool vet -printfuncs=httpErrorf:1,panicif:1,Noticef,Errorf .
go build -o sumatrapdfreader
./sumatrapdfreader
rm sumatrapdfreader

