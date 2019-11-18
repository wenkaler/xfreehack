#!/bin/sh
docker stop xfreehack
docker rm xfreehack
docker run \
  --restart=always \
  --name xfreehack \
  -e TELEGRAM_TOKEN=$TELEGRAM_TOKEN \
  -e PATH_DB=/db/xfree.db \
  -v /db/:/db \
  -d xfreehack:latest
sleep 0.1
docker logs xfreehack -f