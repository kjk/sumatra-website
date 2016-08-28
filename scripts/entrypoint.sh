#!/bin/sh

# sysctl -w fs.file-max=100000
echo "/proc/sys/fs/file-max:"
cat /proc/sys/fs/file-max
echo "ulimit -Hn:"
ulimit -Hn
echo "ulimit -Sn:"
ulimit -Sn
echo "ulimit -Hn 11000:"
ulimit -Hn 11000
echo "ulimit -Sn 11000:"
ulimit -Sn 11000
echo "ulimit -Sn:"
ulimit -Sn

./sumatra_website_linux -addr=:80
