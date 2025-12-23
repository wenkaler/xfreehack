# build binary
FROM golang:1.24-alpine AS build
RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -o /app/xfree \
    -ldflags "-X main.serviceVersion=$VERSION" \
    github.com/wenkaler/xfreehack/cmd

# copy to alpine image
FROM alpine:3.18
WORKDIR /app
COPY --from=build /app/xfree /app/xfree
COPY --from=build /app/config.yaml /app/config.yaml
# Copy migrations if needed, or mount them
COPY --from=build /app/migration /app/migration

RUN apk add --no-cache tzdata ca-certificates
ENV TZ Europe/Moscow
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

CMD ["/app/xfree"]
