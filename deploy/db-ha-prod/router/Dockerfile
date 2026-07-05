FROM golang:1.26.1-alpine
WORKDIR /src
COPY main.go .
RUN go build -trimpath -ldflags="-s -w" -o /usr/local/bin/db-router main.go
ENTRYPOINT ["/usr/local/bin/db-router"]
