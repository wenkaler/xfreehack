#!/bin/sh
docker stop xfreehack
docker rm xfreehack
docker run \
  --restart=always \
  --name xfreehack \
  -e TELEGRAM_TOKEN=$TELEGRAM_TOKEN \
  -v /app:$(pwd)/xfreehack-db \
  -d xfreehack:latest
sleep 0.1
docker logs xfreehack -f