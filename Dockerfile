# docker build -t bpowers/mstat .
FROM golang:1.11-stretch as builder
MAINTAINER Bobby Powers <bobbypowers@gmail.com>

WORKDIR /go/src/github.com/bpowers/mstat
COPY . .

RUN make \
 && make install PREFIX=/usr/local


FROM ubuntu:18.04

COPY --from=builder /usr/local/bin/mstat /usr/local/bin/
