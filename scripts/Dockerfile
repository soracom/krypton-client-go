FROM golang:latest

RUN apt update -y && \
    apt install -y pcscd libpcsclite1 libpcsclite-dev

ENV GOPATH=/go
ENV GOCACHE=/src/.cache