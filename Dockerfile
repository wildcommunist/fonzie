FROM golang:1.18.0-alpine3.14

WORKDIR /cosmos-discord-bot

RUN apk add --no-cache \
  bash \
  git

COPY . .

RUN go build .

FROM alpine:3.14

RUN apk add --no-cache bash

COPY --from=0 /cosmos-discord-bot/fonzie /usr/local/bin/fonzie

CMD fonzie
