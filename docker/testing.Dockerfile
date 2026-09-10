FROM golang:1.25-trixie

RUN go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0

WORKDIR /workspace