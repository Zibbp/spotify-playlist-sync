FROM golang:1 AS build-stage-01

RUN mkdir /app
ADD . /app
WORKDIR /app

RUN CGO_ENABLED=1 GOOS=linux go build -o spotify-convert main.go

FROM debian:12-slim

COPY --from=build-stage-01 /app/spotify-convert .

RUN apt update && apt install -y ca-certificates
RUN update-ca-certificates

ENTRYPOINT ["./spotify-convert"]
