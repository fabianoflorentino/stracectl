#!/bin/sh

URL="https://httpbin.org/get"

while true; do
  wget -q -O /dev/null "$URL" 2>&1
  sleep 3
done
