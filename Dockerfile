FROM golang:1.18.0-alpine3.14

RUN apk add --no-cache \
  bash \
  git

WORKDIR /opt/fonzie

COPY go.mod go.sum *.go help.md Dockerfile ./
COPY chain/ ./chain

RUN go build .

FROM alpine:3.14

RUN apk add --no-cache bash

COPY --from=0 /opt/fonzie/fonzie /usr/local/bin/fonzie

CMD fonzie
