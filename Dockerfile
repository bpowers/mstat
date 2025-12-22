# docker build -t bpowers/mstat .
FROM golang:1.22-alpine as builder

WORKDIR /go/src/github.com/bpowers/mstat
COPY . .

RUN apk add --no-cache make \
 && make \
 && make install PREFIX=/usr/local


FROM alpine:3

COPY --from=builder /usr/local/bin/mstat /usr/local/bin/
