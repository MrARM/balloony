# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o balloony2

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/balloony2 ./
COPY assets ./assets
COPY launchsites.json ./
ENTRYPOINT ["/app/balloony2"]
