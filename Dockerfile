FROM golang:1.25

WORKDIR /app

# RUN go install github.com/air-verse/air@latest

COPY . .
COPY vendor/ ./vendor/

RUN go env -w GOFLAGS="-mod=vendor"
RUN go build -o ws_server

CMD ["/app/ws_server"]
# CMD air
