# syntax=docker/dockerfile:1
FROM golang:1.18-alpine
WORKDIR /app

COPY go.mod ./
COPY go.sum ./
COPY server/main.go ./

RUN go mod download

RUN go build -o /server-main

CMD ["/server-main"]