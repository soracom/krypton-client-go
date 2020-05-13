FROM golang:1.14-stretch

RUN apt update -y && \
    apt install -y pcscd libpcsclite1 libpcsclite-dev

ENV GOPATH=/go
ENV GOCACHE=/src/.cache