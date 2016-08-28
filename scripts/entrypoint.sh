#!/bin/sh

sysctl -w fs.file-max=100000
ulimit -Hn 50000
ulimit -Sn 50000
ulimit -Sn

./sumatra_website_linux -addr=:80
