FROM golang:1.26-alpine3.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o opcua-simulator .

# Stage 2: Run
FROM alpine:3.23

LABEL name="OPC UA Simulator" \
      maintainer="Pavel Konovalov"

WORKDIR /app
COPY --from=builder /app/opcua-simulator .

