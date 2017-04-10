#!/bin/bash
set -u -e -o pipefail

cd www/files

p="http://kjkpub.s3.amazonaws.com/sumatrapdf/rel/"

dlFile() {
  name=${1}
  uri="${p}${name}"
  echo "downloading ${uri} as ${name}"
  curl -O ${uri}
}

dlVer() {
  ver=${1}
  dlFile "SumatraPDF-${ver}-install.exe"
  dlFile "SumatraPDF-${ver}.zip"
  dlFile "SumatraPDF-${ver}-64-install.exe"
  dlFile "SumatraPDF-${ver}-64.zip"
}

dlVer "3.1.1"
dlVer "3.1.2"
dlVer "3.1"
dlVer "3.0"
