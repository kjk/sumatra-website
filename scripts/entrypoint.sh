#!/bin/sh

l1=`cat /proc/sys/fs/file-max`
l2=`ulimit -Sn`
l3=`ulimit -Hn`
echo "file limits: kernel=${l1}, soft ulimit=${l2}, hard ulimit=${l3}"

/app/sumatra_website_linux -production=true -addr=:80
