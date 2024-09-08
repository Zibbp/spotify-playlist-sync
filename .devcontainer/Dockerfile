FROM mcr.microsoft.com/devcontainers/go:1

RUN apt update

RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

RUN apt install sqlite3 -y
