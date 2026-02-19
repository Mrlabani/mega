FROM golang:1.22-alpine

RUN apk add --no-cache mega-cmd bash ca-certificates

WORKDIR /app

COPY go.mod .
COPY go.sum .
COPY main.go .

RUN go mod download
RUN go build -o server

EXPOSE 8080

CMD ["./server"]
