#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

cd /home/sumatrawebsite/www/app/current
exec ./sumatra_website "$@" &>>crash.log
