# syntax=docker/dockerfile:1
FROM golang:1.18-alpine
WORKDIR /app

COPY go.mod ./
COPY go.sum ./
COPY client/main.go ./

RUN go mod download

RUN go build -o /client-main

CMD ["/client-main"]