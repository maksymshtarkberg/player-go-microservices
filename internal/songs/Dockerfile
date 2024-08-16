FROM golang:1.22.2

WORKDIR /usr/src/music-player-go

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN go build -o main ./internal/songs

EXPOSE 8080

CMD ["./main"]
