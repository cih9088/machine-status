# Build image for builder
FROM golang:alpine AS builder
WORKDIR /app

# Set necessary environmet variables needed for our image
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Copy and download dependency using go mod
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the code into the container
COPY . .

# Build the application
RUN go build -o /app/mstat .

FROM ubuntu:20.04
WORKDIR /app

RUN apt-get update && apt-get install -y wget gawk tzdata binutils && rm -rf /var/lib/apt/lists/*

# Default timezone
ENV TZ='Asia/Seoul'
RUN ln -snf /usr/share/zoninfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

COPY scripts ./scripts
COPY web ./web
COPY --from=builder /app/mstat /app/mstat

ENTRYPOINT [ "/app/mstat" ]
