#!/bin/sh
docker stop xfreehack
docker rm xfreehack
docker run \
  --restart=always \
  --name xfreehack \
  -e TELEGRAM_TOKEN={super_secret_token} \
  -v /app:~/xfreehack-db \
  -d xfreehack:latest
sleep 0.1
docker logs xfreehack -f