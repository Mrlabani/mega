FROM golang:1.22

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY main.go .

RUN go build -o server

EXPOSE 8080

CMD ["./server"]
