FROM golang:1.22

# Install dependencies
RUN apt-get update && apt-get install -y \
    mega-cmd \
    ca-certificates \
    bash \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY main.go .

RUN go build -o server

EXPOSE 8080

CMD ["./server"]
